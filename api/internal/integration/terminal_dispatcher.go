package integration

import (
	"context"
	"encoding/json"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/operations"
	"fly-print-cloud/api/internal/websocket"
)

// TerminalDispatcher adapts an accepted integration file into exactly one
// standard print job, but only while the same Edge session remains active.
type TerminalDispatcher struct {
	requests *database.IntegrationPrintRequestRepository
	sessions *database.TerminalSessionRepository
	files    *database.FileRepository
	printers *database.PrinterRepository
	jobs     *database.PrintJobRepository
	manager  *websocket.ConnectionManager
	status   *operations.StatusService
}

func NewTerminalDispatcher(requests *database.IntegrationPrintRequestRepository, sessions *database.TerminalSessionRepository, files *database.FileRepository, printers *database.PrinterRepository, jobs *database.PrintJobRepository, manager *websocket.ConnectionManager, status *operations.StatusService) *TerminalDispatcher {
	return &TerminalDispatcher{requests: requests, sessions: sessions, files: files, printers: printers, jobs: jobs, manager: manager, status: status}
}

func (d *TerminalDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		_ = d.requests.ExpireTickets(time.Now())
		_ = d.ProcessOne()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *TerminalDispatcher) ProcessOne() error {
	request, err := d.requests.ClaimWaitingTerminal(time.Now(), time.Minute)
	if err != nil || request == nil {
		return err
	}
	defer d.requests.ReleaseWaitingTerminal(request.ID)
	matched, err := d.sessions.Matches(request.NodeID, request.TerminalSessionID, request.TerminalTicketHash, "")
	if err != nil || !matched || !d.manager.IsNodeConnected(request.NodeID) {
		return err
	}
	printer, err := d.printers.GetPrinterByID(request.PrinterID)
	if err != nil || printer == nil || !printer.Enabled || printer.EdgeNodeID != request.NodeID {
		return err
	}
	if request.FileID == nil {
		return nil
	}
	file, err := d.files.GetByID(*request.FileID)
	if err != nil || file == nil {
		return err
	}
	options := struct {
		Copies     int    `json:"copies"`
		PaperSize  string `json:"paper_size"`
		ColorMode  string `json:"color_mode"`
		DuplexMode string `json:"duplex_mode"`
	}{Copies: 1}
	if err := json.Unmarshal(request.PrintOptions, &options); err != nil {
		return err
	}
	if options.Copies < 1 {
		options.Copies = 1
	}
	job := &models.PrintJob{
		Name: request.FileName, Status: "pending", PrinterID: request.PrinterID, UserID: request.ExternalUserID, UserName: request.ExternalUserName,
		FilePath: file.FilePath, FileURL: "/api/v1/files/" + file.ID, ContentHash: file.ContentHash, FileSize: file.Size,
		Copies: options.Copies, PaperSize: options.PaperSize, ColorMode: options.ColorMode, DuplexMode: options.DuplexMode, MaxRetries: 3,
		TerminalSessionID: request.TerminalSessionID, TerminalTicketHash: request.TerminalTicketHash, IntegrationRequestID: request.ID,
	}
	if err := d.jobs.CreatePrintJob(job); err != nil {
		return err
	}
	if err := d.manager.DispatchPrintJob(request.NodeID, job); err != nil {
		_ = d.status.ApplyJobResult(job.ID, request.NodeID, request.PrinterID, "failed", "dispatch_failed", map[string]interface{}{"message": "integration job dispatch failed"})
		return err
	}
	if err := d.jobs.MarkDispatched(job.ID); err != nil {
		return err
	}
	return d.requests.MarkDispatched(request.ID, job.ID)
}
