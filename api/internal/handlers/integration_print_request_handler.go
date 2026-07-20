package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/integration"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"

	"github.com/gin-gonic/gin"
)

type IntegrationPrintRequestHandler struct {
	providers *database.IntegrationProviderRepository
	requests  *database.IntegrationPrintRequestRepository
	cipher    *security.ClientSecretCipher
	nonces    *integration.NonceStore
}

func NewIntegrationPrintRequestHandler(providers *database.IntegrationProviderRepository, requests *database.IntegrationPrintRequestRepository, cipher *security.ClientSecretCipher, nonces *integration.NonceStore) *IntegrationPrintRequestHandler {
	return &IntegrationPrintRequestHandler{providers: providers, requests: requests, cipher: cipher, nonces: nonces}
}

type integrationPrintRequestBody struct {
	ExternalOrderID  string `json:"external_order_id"`
	ExternalUserID   string `json:"external_user_id"`
	ExternalUserName string `json:"external_user_name"`
	TerminalTicket   string `json:"terminal_ticket"`
	File             struct {
		URL       string `json:"url"`
		ExpiresAt string `json:"expires_at"`
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		MIMEType  string `json:"mime_type"`
		SHA256    string `json:"sha256"`
	} `json:"file"`
	PrintOptions struct {
		Copies     int    `json:"copies"`
		PaperSize  string `json:"paper_size"`
		ColorMode  string `json:"color_mode"`
		DuplexMode string `json:"duplex_mode"`
	} `json:"print_options"`
	Metadata json.RawMessage `json:"metadata"`
}

var sha256HexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (h *IntegrationPrintRequestHandler) Create(c *gin.Context) {
	provider, rawBody, ok := h.authenticate(c)
	if !ok {
		return
	}
	var input integrationPrintRequestBody
	if err := json.Unmarshal(rawBody, &input); err != nil {
		BadRequestResponse(c, "invalid integration request JSON")
		return
	}
	if err := validateIntegrationInput(input, provider); err != nil {
		BadRequestResponse(c, err.Error())
		return
	}
	fileExpiresAt, _ := time.Parse(time.RFC3339, input.File.ExpiresAt)
	printOptions, _ := json.Marshal(input.PrintOptions)
	metadata := input.Metadata
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	requestHash := sha256.Sum256(rawBody)
	request := &models.IntegrationPrintRequest{
		ProviderCode: provider.Code, ExternalOrderID: input.ExternalOrderID, RequestHash: hex.EncodeToString(requestHash[:]),
		TerminalTicketHash: ticketHash(input.TerminalTicket), ExternalUserID: input.ExternalUserID, ExternalUserName: input.ExternalUserName,
		FileURL: input.File.URL, FileName: input.File.Name, FileSize: input.File.Size, MimeType: input.File.MIMEType,
		FileSHA256: input.File.SHA256, PrintOptions: printOptions, Metadata: metadata,
		FileExpiresAt: fileExpiresAt,
		ExpiresAt:     time.Now().Add(terminalTicketTTL),
	}
	created, existing, err := h.requests.CreateOrGet(request, time.Now())
	if err == database.ErrIntegrationOrderConflict {
		ErrorResponse(c, http.StatusConflict, "external_order_id already exists with different parameters")
		return
	}
	if err == database.ErrIntegrationTicketUnavailable {
		ErrorResponse(c, http.StatusConflict, "terminal ticket is unavailable for this provider")
		return
	}
	if err != nil {
		InternalErrorResponse(c, "failed to accept integration print request")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"request_id": created.ID, "status": created.Status, "idempotent": existing})
}

func (h *IntegrationPrintRequestHandler) Get(c *gin.Context) {
	provider, _, ok := h.authenticate(c)
	if !ok {
		return
	}
	request, err := h.requests.Get(provider.Code, c.Param("request_id"))
	if err != nil {
		InternalErrorResponse(c, "failed to get integration print request")
		return
	}
	if request == nil {
		NotFoundResponse(c, "integration print request not found")
		return
	}
	SuccessResponse(c, request)
}

// authenticate checks all transport properties before parsing JSON. It never
// logs a ticket, signature, URL, raw body, or provider secret.
func (h *IntegrationPrintRequestHandler) authenticate(c *gin.Context) (*models.IntegrationProvider, []byte, bool) {
	providerCode := c.Param("provider")
	if c.GetHeader("X-FP-Client") != providerCode || !providerCodePattern.MatchString(providerCode) {
		UnauthorizedResponse(c, "provider identity does not match request path")
		return nil, nil, false
	}
	provider, err := h.providers.Get(providerCode, true)
	if err != nil || provider == nil || !provider.Enabled {
		UnauthorizedResponse(c, "provider is unavailable")
		return nil, nil, false
	}
	if !providerAllowsIP(provider.AllowedIPCIDRs, c.ClientIP()) {
		ForbiddenResponse(c, "source IP is not allowed")
		return nil, nil, false
	}
	if h.nonces == nil {
		InternalErrorResponse(c, "integration nonce store is unavailable")
		return nil, nil, false
	}
	rawBody, err := c.GetRawData()
	if err != nil {
		BadRequestResponse(c, "could not read request body")
		return nil, nil, false
	}
	secret, err := h.cipher.Decrypt(provider.InboundSecretEncrypted)
	if err != nil {
		InternalErrorResponse(c, "provider credential is unavailable")
		return nil, nil, false
	}
	if err := integration.VerifySignature(secret, c.GetHeader("X-FP-Signature"), c.Request.Method, c.Request.URL.Path, c.GetHeader("X-FP-Timestamp"), c.GetHeader("X-FP-Nonce"), rawBody, time.Now()); err != nil {
		UnauthorizedResponse(c, "invalid integration signature")
		return nil, nil, false
	}
	accepted, err := h.nonces.Use(c.Request.Context(), provider.Code, c.GetHeader("X-FP-Nonce"))
	if err != nil {
		InternalErrorResponse(c, "integration nonce store is unavailable")
		return nil, nil, false
	}
	if !accepted {
		ErrorResponse(c, http.StatusConflict, "integration nonce was already used")
		return nil, nil, false
	}
	return provider, rawBody, true
}

func validateIntegrationInput(input integrationPrintRequestBody, provider *models.IntegrationProvider) error {
	if input.ExternalOrderID == "" || input.TerminalTicket == "" || input.File.URL == "" || input.File.Name == "" || input.File.Size <= 0 || !sha256HexPattern.MatchString(input.File.SHA256) {
		return fmt.Errorf("external_order_id, terminal_ticket, and complete file metadata are required")
	}
	fileExpiresAt, err := time.Parse(time.RFC3339, input.File.ExpiresAt)
	if err != nil {
		return fmt.Errorf("file.expires_at must be RFC3339")
	}
	if !fileExpiresAt.After(time.Now()) {
		return fmt.Errorf("file.expires_at must be in the future")
	}
	if input.File.Size > provider.MaxFileSize || !csvContains(provider.AllowedMIMETypes, input.File.MIMEType) {
		return fmt.Errorf("file violates provider size or MIME policy")
	}
	if input.PrintOptions.Copies < 1 || input.PrintOptions.Copies > 99 {
		return fmt.Errorf("print_options.copies must be between 1 and 99")
	}
	return nil
}

func providerAllowsIP(cidrs, source string) bool {
	address := net.ParseIP(source)
	if address == nil {
		return false
	}
	for _, rawCIDR := range splitCSV(cidrs) {
		_, network, err := net.ParseCIDR(rawCIDR)
		if err == nil && network.Contains(address) {
			return true
		}
	}
	return false
}

func csvContains(values, needle string) bool {
	for _, value := range splitCSV(values) {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
