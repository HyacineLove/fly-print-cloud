package handlers

import (
	"errors"
	"strings"

	"fly-print-cloud/api/internal/business"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"

	"github.com/gin-gonic/gin"
)

type opsContactSettings interface {
	Current() (business.Settings, error)
}

// OpsContactHandler manages display-only ops contact profiles (not login users).
type OpsContactHandler struct {
	repo     *database.OpsContactRepository
	settings opsContactSettings
}

func NewOpsContactHandler(repo *database.OpsContactRepository, settings opsContactSettings) *OpsContactHandler {
	return &OpsContactHandler{repo: repo, settings: settings}
}

type opsContactRequest struct {
	Name    string   `json:"name"`
	Phone   string   `json:"phone"`
	Enabled *bool    `json:"enabled"`
	NodeIDs []string `json:"node_ids"`
}

type opsContactNodesRequest struct {
	NodeIDs []string `json:"node_ids"`
}

type opsContactEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *OpsContactHandler) List(c *gin.Context) {
	page, pageSize, offset := ParsePaginationParams(c)
	filter := database.OpsContactListFilter{
		Search: strings.TrimSpace(c.Query("search")),
		NodeID: strings.TrimSpace(c.Query("node_id")),
		Offset: offset,
		Limit:  pageSize,
	}
	if enabledRaw := c.Query("enabled"); enabledRaw != "" {
		enabled := enabledRaw == "true" || enabledRaw == "1"
		filter.Enabled = &enabled
	}

	contacts, total, err := h.repo.List(filter)
	if err != nil {
		InternalErrorResponse(c, "failed to list ops contacts")
		return
	}
	PaginatedSuccessResponse(c, contacts, total, page, pageSize)
}

func (h *OpsContactHandler) Get(c *gin.Context) {
	contact, err := h.repo.Get(c.Param("id"))
	if err != nil {
		if errors.Is(err, database.ErrOpsContactNotFound) {
			NotFoundResponse(c, "ops contact not found")
			return
		}
		InternalErrorResponse(c, "failed to get ops contact")
		return
	}
	SuccessResponse(c, contact)
}

func (h *OpsContactHandler) Create(c *gin.Context) {
	var request opsContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		BadRequestResponse(c, "invalid ops contact")
		return
	}
	name, phone, err := normalizeOpsContact(request.Name, request.Phone)
	if err != nil {
		BadRequestResponse(c, err.Error())
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	contact := &models.OpsContact{Name: name, Phone: phone, Enabled: enabled}
	if err := h.repo.Create(contact); err != nil {
		InternalErrorResponse(c, "failed to create ops contact")
		return
	}
	if request.NodeIDs != nil {
		if err := h.replaceNodes(contact.ID, request.NodeIDs); err != nil {
			_ = h.repo.SoftDelete(contact.ID)
			h.writeBindingError(c, err)
			return
		}
		contact, err = h.repo.Get(contact.ID)
		if err != nil {
			InternalErrorResponse(c, "failed to load created ops contact")
			return
		}
	}
	CreatedResponse(c, contact)
}

func (h *OpsContactHandler) Update(c *gin.Context) {
	current, err := h.repo.Get(c.Param("id"))
	if err != nil {
		if errors.Is(err, database.ErrOpsContactNotFound) {
			NotFoundResponse(c, "ops contact not found")
			return
		}
		InternalErrorResponse(c, "failed to get ops contact")
		return
	}

	var request opsContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		BadRequestResponse(c, "invalid ops contact")
		return
	}
	name, phone, err := normalizeOpsContact(request.Name, request.Phone)
	if err != nil {
		BadRequestResponse(c, err.Error())
		return
	}
	current.Name = name
	current.Phone = phone
	if request.Enabled != nil {
		current.Enabled = *request.Enabled
	}
	if err := h.repo.Update(current); err != nil {
		InternalErrorResponse(c, "failed to update ops contact")
		return
	}
	if request.NodeIDs != nil {
		if err := h.replaceNodes(current.ID, request.NodeIDs); err != nil {
			h.writeBindingError(c, err)
			return
		}
	}
	updated, err := h.repo.Get(current.ID)
	if err != nil {
		InternalErrorResponse(c, "failed to load updated ops contact")
		return
	}
	SuccessResponse(c, updated)
}

func (h *OpsContactHandler) Delete(c *gin.Context) {
	if err := h.repo.SoftDelete(c.Param("id")); err != nil {
		if errors.Is(err, database.ErrOpsContactNotFound) {
			NotFoundResponse(c, "ops contact not found")
			return
		}
		InternalErrorResponse(c, "failed to delete ops contact")
		return
	}
	SuccessResponse(c, gin.H{"message": "ops contact deleted"})
}

func (h *OpsContactHandler) UpdateEnabled(c *gin.Context) {
	var request opsContactEnabledRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		BadRequestResponse(c, "invalid enabled payload")
		return
	}
	contact, err := h.repo.UpdateEnabled(c.Param("id"), request.Enabled)
	if err != nil {
		if errors.Is(err, database.ErrOpsContactNotFound) {
			NotFoundResponse(c, "ops contact not found")
			return
		}
		InternalErrorResponse(c, "failed to update ops contact enabled")
		return
	}
	SuccessResponse(c, contact)
}

func (h *OpsContactHandler) ReplaceNodes(c *gin.Context) {
	var request opsContactNodesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		BadRequestResponse(c, "invalid node_ids payload")
		return
	}
	if err := h.replaceNodes(c.Param("id"), request.NodeIDs); err != nil {
		h.writeBindingError(c, err)
		return
	}
	contact, err := h.repo.Get(c.Param("id"))
	if err != nil {
		if errors.Is(err, database.ErrOpsContactNotFound) {
			NotFoundResponse(c, "ops contact not found")
			return
		}
		InternalErrorResponse(c, "failed to load ops contact")
		return
	}
	SuccessResponse(c, contact)
}

// ListSelfContacts returns enabled contacts bound to the authenticated Edge node.
func (h *OpsContactHandler) ListSelfContacts(c *gin.Context) {
	nodeID := c.GetString("node_id")
	if nodeID == "" {
		UnauthorizedResponse(c, "节点身份无效")
		return
	}
	contacts, err := h.repo.ListPublicForNode(nodeID)
	if err != nil {
		InternalErrorResponse(c, "failed to list node contacts")
		return
	}
	SuccessResponse(c, contacts)
}

func (h *OpsContactHandler) replaceNodes(contactID string, nodeIDs []string) error {
	maxPerNode := business.DefaultMaxContactsPerNode
	if h.settings != nil {
		if settings, err := h.settings.Current(); err == nil && settings.MaxContactsPerNode > 0 {
			maxPerNode = settings.MaxContactsPerNode
		}
	}
	return h.repo.ReplaceNodeBindings(contactID, nodeIDs, maxPerNode)
}

func (h *OpsContactHandler) writeBindingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, database.ErrOpsContactNotFound):
		NotFoundResponse(c, "ops contact not found")
	case errors.Is(err, database.ErrOpsContactNodeNotFound):
		BadRequestResponse(c, "edge node not found")
	case errors.Is(err, database.ErrNodeContactLimitExceeded):
		BadRequestResponse(c, err.Error())
	default:
		InternalErrorResponse(c, "failed to update node bindings")
	}
}

func normalizeOpsContact(name, phone string) (string, string, error) {
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)
	if name == "" || len(name) > 100 {
		return "", "", errors.New("name is required and must be at most 100 characters")
	}
	if phone == "" || len(phone) > 40 {
		return "", "", errors.New("phone is required and must be at most 40 characters")
	}
	return name, phone, nil
}
