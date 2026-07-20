package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"

	"github.com/gin-gonic/gin"
)

const terminalTicketTTL = 5 * time.Minute

type TerminalTicketHandler struct {
	tickets      *database.TerminalTicketRepository
	printers     *database.PrinterRepository
	edgeNodes    *database.EdgeNodeRepository
	providers    *database.IntegrationProviderRepository
	uploadSessions *database.TerminalUploadSessionRepository
	tokens       *security.TokenManager
	entryBaseURL string
}

func NewTerminalTicketHandler(tickets *database.TerminalTicketRepository, printers *database.PrinterRepository, edgeNodes *database.EdgeNodeRepository, providers *database.IntegrationProviderRepository, uploadSessions *database.TerminalUploadSessionRepository, tokens *security.TokenManager, entryBaseURL string) *TerminalTicketHandler {
	return &TerminalTicketHandler{tickets: tickets, printers: printers, edgeNodes: edgeNodes, providers: providers, uploadSessions: uploadSessions, tokens: tokens, entryBaseURL: strings.TrimRight(entryBaseURL, "/")}
}

type issueTerminalTicketRequest struct {
	PrinterID         string `json:"printer_id" binding:"required"`
	TerminalSessionID string `json:"terminal_session_id" binding:"required,max=128"`
}

func (h *TerminalTicketHandler) IssueForSelf(c *gin.Context) {
	if h.entryBaseURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "terminal_entry_not_configured"})
		return
	}
	var req issueTerminalTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}
	nodeID := c.GetString("node_id")
	if nodeID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "edge_node_identity_missing"})
		return
	}
	node, err := h.edgeNodes.GetEdgeNodeByID(nodeID)
	if err != nil || node == nil || !node.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "node_unavailable"})
		return
	}
	printer, err := h.printers.GetPrinterByID(req.PrinterID)
	if err != nil || printer == nil || printer.EdgeNodeID != nodeID || !printer.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "printer_unavailable"})
		return
	}
	raw, err := newTerminalTicket()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terminal_ticket_generation_failed"})
		return
	}
	ticket := &models.TerminalTicket{TicketHash: ticketHash(raw), NodeID: nodeID, PrinterID: req.PrinterID, TerminalSessionID: req.TerminalSessionID, ExpiresAt: time.Now().Add(terminalTicketTTL)}
	if err := h.tickets.Create(ticket); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terminal_ticket_creation_failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"terminal_ticket": raw, "entry_url": h.entryBaseURL + "/entry?terminal_ticket=" + raw, "expires_at": ticket.ExpiresAt})
}

func (h *TerminalTicketHandler) EntryPage(c *gin.Context) {
	raw := c.Query("terminal_ticket")
	if raw == "" {
		c.String(http.StatusBadRequest, "invalid terminal ticket")
		return
	}
	if _, err := h.tickets.GetValidByHash(ticketHash(raw), time.Now()); err != nil {
		c.String(http.StatusGone, "terminal ticket expired or unavailable")
		return
	}
	providers, err := h.providers.List()
	if err != nil { c.String(http.StatusServiceUnavailable, "entry temporarily unavailable"); return }
	ticket := html.EscapeString(raw)
	buttons := "<button data-entry=official>官方打印</button>"
	for _, provider := range providers {
		if provider.Enabled && provider.EntryVisible {
			buttons += "<button data-entry='" + html.EscapeString(provider.Code) + "'>" + html.EscapeString(provider.DisplayName) + "</button>"
		}
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!doctype html><meta name=viewport content='width=device-width,initial-scale=1'><title>FlyPrint</title><main><h1>请选择打印入口</h1>"+buttons+"<script>document.querySelectorAll('[data-entry]').forEach(b=>b.onclick=async()=>{const r=await fetch('/api/v1/public/terminal-entry/select',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({terminal_ticket:'"+ticket+"',entry:b.dataset.entry})});const d=await r.json();if(d.redirect_url)location.href=d.redirect_url;else alert('入口暂不可用，请重新扫码')});</script></main>"))
}

type selectTerminalEntryRequest struct {
	TerminalTicket string `json:"terminal_ticket" binding:"required"`
	Entry          string `json:"entry" binding:"required"`
}

func (h *TerminalTicketHandler) SelectEntry(c *gin.Context) {
	var req selectTerminalEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil || !providerCodePattern.MatchString(req.Entry) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_terminal_entry"})
		return
	}
	var provider *models.IntegrationProvider
	if req.Entry != "official" {
		var err error
		provider, err = h.providers.Get(req.Entry, false)
		if err != nil || provider == nil || !provider.Enabled || !provider.EntryVisible { c.JSON(http.StatusBadRequest, gin.H{"error":"terminal_entry_unavailable"}); return }
	}
	ticket, err := h.tickets.Select(ticketHash(req.TerminalTicket), req.Entry, time.Now())
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "terminal_ticket_locked_or_expired"})
		return
	}
	if req.Entry == "official" {
		token, expiresAt, err := h.tokens.GenerateUploadToken(ticket.NodeID, ticket.PrinterID)
		if err != nil || h.uploadSessions.Create(token, ticket.TicketHash, ticket.NodeID, ticket.PrinterID, ticket.TerminalSessionID, expiresAt) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error":"official_upload_unavailable"}); return
		}
		query := url.Values{"token": {token}, "node_id": {ticket.NodeID}, "printer_id": {ticket.PrinterID}}
		c.JSON(http.StatusOK, gin.H{"redirect_url":"/upload?" + query.Encode()})
		return
	}
	redirect, err := url.Parse(provider.EntryURL)
	if err != nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error":"terminal_entry_unavailable"}); return }
	query := redirect.Query(); query.Set("terminal_ticket", req.TerminalTicket); redirect.RawQuery = query.Encode()
	c.JSON(http.StatusOK, gin.H{"redirect_url":redirect.String()})
}

func newTerminalTicket() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", err }
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func ticketHash(raw string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) }
