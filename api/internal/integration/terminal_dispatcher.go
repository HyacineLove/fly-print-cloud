package integration

import (
	"context"
	"encoding/json"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/websocket"
)

// TerminalDispatcher delivers an integration file into the existing Edge
// preview flow. It never creates or prints a job; only an explicit Edge user
// confirmation may bridge the request into print_jobs.
type TerminalDispatcher struct {
	requests *database.IntegrationPrintRequestRepository
	sessions *database.TerminalSessionRepository
	files    *database.FileRepository
	printers *database.PrinterRepository
	manager  *websocket.ConnectionManager
}

func NewTerminalDispatcher(requests *database.IntegrationPrintRequestRepository, sessions *database.TerminalSessionRepository, files *database.FileRepository, printers *database.PrinterRepository, manager *websocket.ConnectionManager) *TerminalDispatcher {
	return &TerminalDispatcher{requests: requests, sessions: sessions, files: files, printers: printers, manager: manager}
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

	options := map[string]interface{}{}
	if err := json.Unmarshal(request.PrintOptions, &options); err != nil {
		return err
	}
	payload := websocket.PreviewFilePayload{
		FileID: file.ID, FileURL: "/api/v1/files/" + file.ID, FileName: file.OriginalName,
		FileSize: file.Size, FileType: file.MimeType, ContentHash: file.ContentHash,
		PrintOptions: options, TerminalSessionID: request.TerminalSessionID,
		TerminalTicketHash: request.TerminalTicketHash, IntegrationRequestID: request.ID,
	}
	if err := d.manager.DispatchIntegrationPreview(request.NodeID, payload); err != nil {
		return err
	}
	return d.requests.MarkPreviewSent(request.ID, time.Now())
}
