package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fly-print-cloud/api/internal/models"

	"github.com/google/uuid"
)

var ErrIntegrationOrderConflict = errors.New("integration external order parameters conflict")
var ErrIntegrationTicketUnavailable = errors.New("terminal ticket is not selected for this provider")
var ErrIntegrationConfirmationMismatch = errors.New("integration confirmation does not match the active terminal session")

type IntegrationPrintRequestRepository struct{ db *DB }

func NewIntegrationPrintRequestRepository(db *DB) *IntegrationPrintRequestRepository {
	return &IntegrationPrintRequestRepository{db: db}
}

// CreateOrGet atomically enforces order idempotency and consumes the selected
// terminal ticket. A different order can never reuse a selected/consumed ticket.
func (r *IntegrationPrintRequestRepository) CreateOrGet(request *models.IntegrationPrintRequest, now time.Time) (*models.IntegrationPrintRequest, bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	existing, err := scanIntegrationRequest(tx.QueryRow(integrationRequestSelect+` WHERE provider_code=$1 AND external_order_id=$2`, request.ProviderCode, request.ExternalOrderID))
	if err == nil {
		if existing.RequestHash != request.RequestHash {
			return nil, false, ErrIntegrationOrderConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return existing, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("find existing integration request: %w", err)
	}

	var nodeID, printerID, terminalSessionID string
	err = tx.QueryRow(`UPDATE terminal_tickets SET status='consumed', consumed_at=$3
		WHERE ticket_hash=$1 AND status='selected' AND selected_entry=$2 AND expires_at>$3
		RETURNING node_id,printer_id,terminal_session_id`, request.TerminalTicketHash, request.ProviderCode, now).Scan(&nodeID, &printerID, &terminalSessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, ErrIntegrationTicketUnavailable
	}
	if err != nil {
		return nil, false, fmt.Errorf("consume terminal ticket: %w", err)
	}
	request.NodeID, request.PrinterID, request.TerminalSessionID = nodeID, printerID, terminalSessionID

	err = tx.QueryRow(`INSERT INTO integration_print_requests (
		provider_code,external_order_id,request_hash,terminal_ticket_hash,node_id,printer_id,terminal_session_id,
		external_user_id,external_user_name,file_url,file_name,file_size,mime_type,file_sha256,file_expires_at,print_options,metadata,status,expires_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,'waiting_file',$18)
	RETURNING id,created_at,updated_at`, request.ProviderCode, request.ExternalOrderID, request.RequestHash, request.TerminalTicketHash,
		request.NodeID, request.PrinterID, request.TerminalSessionID, request.ExternalUserID, request.ExternalUserName,
		request.FileURL, request.FileName, request.FileSize, request.MimeType, request.FileSHA256, request.FileExpiresAt, request.PrintOptions, request.Metadata, request.ExpiresAt,
	).Scan(&request.ID, &request.CreatedAt, &request.UpdatedAt)
	if err != nil {
		return nil, false, fmt.Errorf("create integration request: %w", err)
	}
	request.Status = "waiting_file"
	if err := enqueueIntegrationCallbackTx(tx, request.ID, request.Status, "", ""); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return request, false, nil
}

func (r *IntegrationPrintRequestRepository) Get(providerCode, requestID string) (*models.IntegrationPrintRequest, error) {
	request, err := scanIntegrationRequest(r.db.QueryRow(integrationRequestSelect+` WHERE provider_code=$1 AND id=$2`, providerCode, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

// ClaimWaitingFile uses SKIP LOCKED plus a lease, allowing several Cloud
// instances to share the worker without processing the same request twice.
func (r *IntegrationPrintRequestRepository) ClaimWaitingFile(now time.Time, lease time.Duration) (*models.IntegrationPrintRequest, error) {
	query := `WITH candidate AS (
		SELECT id FROM integration_print_requests
		WHERE status='waiting_file' AND (worker_lease_until IS NULL OR worker_lease_until < $1)
		ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE integration_print_requests AS request
	SET worker_lease_until=$2 FROM candidate WHERE request.id=candidate.id
	RETURNING request.id,request.provider_code,request.external_order_id,request.request_hash,request.terminal_ticket_hash,request.node_id,request.printer_id,request.terminal_session_id,
	request.external_user_id,request.external_user_name,request.file_url,request.file_name,request.file_size,request.mime_type,request.file_sha256,request.file_expires_at,request.print_options,request.metadata,request.file_id,request.print_job_id,
	request.preview_sent_at,request.status,request.error_code,request.error_message,request.expires_at,request.created_at,request.updated_at`
	request, err := scanIntegrationRequest(r.db.QueryRow(query, now, now.Add(lease)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

func (r *IntegrationPrintRequestRepository) MarkFileReady(requestID, fileID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE integration_print_requests SET file_id=$2::uuid,status='waiting_terminal',worker_lease_until=NULL
		WHERE id=$1 AND status='waiting_file'`, requestID, fileID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("integration request is no longer waiting for a file")
	}
	if err := enqueueIntegrationCallbackTx(tx, requestID, "waiting_terminal", "", ""); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *IntegrationPrintRequestRepository) MarkFileFailed(requestID, code, message string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE integration_print_requests SET status='failed',error_code=$2,error_message=$3,worker_lease_until=NULL
		WHERE id=$1 AND status='waiting_file'`, requestID, code, message)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return nil
	}
	if err := enqueueIntegrationCallbackTx(tx, requestID, "failed", code, message); err != nil {
		return err
	}
	return tx.Commit()
}

// CancelForNodeTx cancels only non-terminal integration requests while a node
// is being removed. Each visible state change gets its callback outbox row in
// the same transaction so providers do not observe an unreported cancellation.
func (r *IntegrationPrintRequestRepository) CancelForNodeTx(tx *Tx, nodeID string) error {
	rows, err := tx.Query(`UPDATE integration_print_requests
		SET status='cancelled',error_code='node_deleted',error_message='target node was deleted'
		WHERE node_id=$1 AND status IN ('accepted','waiting_file','waiting_terminal','dispatched','printing')
		RETURNING id::text`, nodeID)
	if err != nil {
		return err
	}
	var requestIDs []string
	for rows.Next() {
		var requestID string
		if err := rows.Scan(&requestID); err != nil {
			rows.Close()
			return err
		}
		requestIDs = append(requestIDs, requestID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, requestID := range requestIDs {
		if err := enqueueIntegrationCallbackTx(tx.Tx, requestID, "cancelled", "node_deleted", "target node was deleted"); err != nil {
			return err
		}
	}
	return nil
}

func (r *IntegrationPrintRequestRepository) ClaimWaitingTerminal(now time.Time, lease time.Duration) (*models.IntegrationPrintRequest, error) {
	query := `WITH candidate AS (
		SELECT request.id FROM integration_print_requests request WHERE request.status='waiting_terminal' AND request.expires_at>$1
		AND request.print_job_id IS NULL
		AND (request.preview_sent_at IS NULL OR request.preview_sent_at < $1 - interval '5 seconds')
		AND NOT EXISTS (SELECT 1 FROM edge_terminal_sessions session
			WHERE session.node_id=request.node_id AND session.integration_request_id=request.id)
		AND (request.worker_lease_until IS NULL OR request.worker_lease_until<$1) ORDER BY request.created_at FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE integration_print_requests AS request SET worker_lease_until=$2 FROM candidate WHERE request.id=candidate.id
	RETURNING request.id,request.provider_code,request.external_order_id,request.request_hash,request.terminal_ticket_hash,request.node_id,request.printer_id,request.terminal_session_id,
	request.external_user_id,request.external_user_name,request.file_url,request.file_name,request.file_size,request.mime_type,request.file_sha256,request.file_expires_at,request.print_options,request.metadata,request.file_id,request.print_job_id,
	request.preview_sent_at,request.status,request.error_code,request.error_message,request.expires_at,request.created_at,request.updated_at`
	request, err := scanIntegrationRequest(r.db.QueryRow(query, now, now.Add(lease)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

func (r *IntegrationPrintRequestRepository) MarkPreviewSent(requestID string, sentAt time.Time) error {
	_, err := r.db.Exec(`UPDATE integration_print_requests SET preview_sent_at=$2,worker_lease_until=NULL
		WHERE id=$1 AND status='waiting_terminal' AND print_job_id IS NULL`, requestID, sentAt)
	return err
}

func (r *IntegrationPrintRequestRepository) ReleaseWaitingTerminal(requestID string) error {
	_, err := r.db.Exec(`UPDATE integration_print_requests SET worker_lease_until=NULL WHERE id=$1 AND status='waiting_terminal'`, requestID)
	return err
}

func (r *IntegrationPrintRequestRepository) MarkDispatched(requestID, jobID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE integration_print_requests SET status='dispatched',worker_lease_until=NULL
		WHERE id=$1 AND status='waiting_terminal' AND print_job_id=$2::uuid`, requestID, jobID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("integration request is no longer waiting for terminal")
	}
	if err := enqueueIntegrationCallbackTx(tx, requestID, "dispatched", "", ""); err != nil {
		return err
	}
	return tx.Commit()
}

type IntegrationPrintConfirmation struct {
	RequestID, NodeID, FileID, PrinterID  string
	TerminalSessionID, TerminalTicketHash string
	Copies                                int
	PaperSize, ColorMode, DuplexMode      string
}

// ConfirmAndCreateJob is the only bridge from an integration request into the
// standard print_jobs table. The terminal proof and job reservation are
// checked and written in one transaction, so repeated confirmations cannot
// create a second physical job.
func (r *IntegrationPrintRequestRepository) ConfirmAndCreateJob(input IntegrationPrintConfirmation) (*models.PrintJob, bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var request models.IntegrationPrintRequest
	var file models.File
	var existingJob sql.NullString
	var printerNode, boundRequest string
	var printerEnabled bool
	err = tx.QueryRow(`SELECT request.node_id,request.printer_id::text,request.terminal_session_id,request.terminal_ticket_hash,
		request.external_user_id,request.external_user_name,request.file_id::text,request.print_job_id::text,
		file.original_name,file.file_path,file.size,file.content_hash,printer.edge_node_id,printer.enabled,
		COALESCE(session.integration_request_id::text,'')
		FROM integration_print_requests request
		JOIN files file ON file.id=request.file_id
		JOIN printers printer ON printer.id=request.printer_id
		JOIN edge_terminal_sessions session ON session.node_id=request.node_id
		WHERE request.id=$1::uuid AND request.status='waiting_terminal' AND request.expires_at>CURRENT_TIMESTAMP
		FOR UPDATE OF request`, input.RequestID).Scan(
		&request.NodeID, &request.PrinterID, &request.TerminalSessionID, &request.TerminalTicketHash,
		&request.ExternalUserID, &request.ExternalUserName, &file.ID, &existingJob,
		&file.OriginalName, &file.FilePath, &file.Size, &file.ContentHash, &printerNode, &printerEnabled, &boundRequest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, ErrIntegrationConfirmationMismatch
	}
	if err != nil {
		return nil, false, err
	}
	if existingJob.Valid {
		return nil, false, nil
	}
	if request.NodeID != input.NodeID || request.PrinterID != input.PrinterID || file.ID != input.FileID ||
		request.TerminalSessionID != input.TerminalSessionID || request.TerminalTicketHash != input.TerminalTicketHash ||
		printerNode != input.NodeID || !printerEnabled || boundRequest != input.RequestID {
		return nil, false, ErrIntegrationConfirmationMismatch
	}
	if input.Copies < 1 {
		input.Copies = 1
	}
	job := &models.PrintJob{ID: uuid.NewString(), Name: file.OriginalName, Status: "pending", PrinterID: input.PrinterID,
		UserID: request.ExternalUserID, UserName: request.ExternalUserName, FilePath: file.FilePath,
		FileURL: "/api/v1/files/" + file.ID, ContentHash: file.ContentHash, FileSize: file.Size,
		Copies: input.Copies, PaperSize: input.PaperSize, ColorMode: input.ColorMode, DuplexMode: input.DuplexMode,
		MaxRetries: 3, TerminalSessionID: input.TerminalSessionID, TerminalTicketHash: input.TerminalTicketHash,
		IntegrationRequestID: input.RequestID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_, err = tx.Exec(`INSERT INTO print_jobs(id,name,status,printer_id,user_id,user_name,file_path,file_url,content_hash,file_size,page_count,
		copies,paper_size,color_mode,duplex_mode,retry_count,max_retries,created_at,updated_at)
		VALUES($1,$2,$3,$4::uuid,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,0,$11,$12,$13,$14,0,$15,$16,$17)`,
		job.ID, job.Name, job.Status, job.PrinterID, job.UserID, job.UserName, job.FilePath, job.FileURL, job.ContentHash, job.FileSize,
		job.Copies, job.PaperSize, job.ColorMode, job.DuplexMode, job.MaxRetries, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return nil, false, err
	}
	result, err := tx.Exec(`UPDATE integration_print_requests SET print_job_id=$2::uuid WHERE id=$1::uuid AND print_job_id IS NULL`, input.RequestID, job.ID)
	if err != nil {
		return nil, false, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return nil, false, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return job, true, nil
}

func (r *IntegrationPrintRequestRepository) ExpireTickets(now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`UPDATE integration_print_requests SET status='expired',error_code='terminal_ticket_expired',error_message='terminal ticket expired'
		WHERE status IN ('waiting_file','waiting_terminal') AND expires_at <= $1 RETURNING id::text`, now)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var requestID string
		if err := rows.Scan(&requestID); err != nil {
			return err
		}
		if err := enqueueIntegrationCallbackTx(tx, requestID, "expired", "terminal_ticket_expired", "terminal ticket expired"); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

const integrationRequestSelect = `SELECT id,provider_code,external_order_id,request_hash,terminal_ticket_hash,node_id,printer_id,terminal_session_id,
	external_user_id,external_user_name,file_url,file_name,file_size,mime_type,file_sha256,file_expires_at,print_options,metadata,file_id,print_job_id,
	preview_sent_at,status,error_code,error_message,expires_at,created_at,updated_at FROM integration_print_requests`

func scanIntegrationRequest(scanner interface{ Scan(...any) error }) (*models.IntegrationPrintRequest, error) {
	request := &models.IntegrationPrintRequest{}
	var externalUserID, externalUserName sql.NullString
	var fileID, printJobID, errorCode, errorMessage sql.NullString
	err := scanner.Scan(&request.ID, &request.ProviderCode, &request.ExternalOrderID, &request.RequestHash, &request.TerminalTicketHash,
		&request.NodeID, &request.PrinterID, &request.TerminalSessionID, &externalUserID, &externalUserName, &request.FileURL,
		&request.FileName, &request.FileSize, &request.MimeType, &request.FileSHA256, &request.FileExpiresAt, &request.PrintOptions, &request.Metadata,
		&fileID, &printJobID, &request.PreviewSentAt, &request.Status, &errorCode, &errorMessage, &request.ExpiresAt, &request.CreatedAt, &request.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if externalUserID.Valid {
		request.ExternalUserID = externalUserID.String
	}
	if externalUserName.Valid {
		request.ExternalUserName = externalUserName.String
	}
	if fileID.Valid {
		request.FileID = &fileID.String
	}
	if printJobID.Valid {
		request.PrintJobID = &printJobID.String
	}
	if errorCode.Valid {
		request.ErrorCode = &errorCode.String
	}
	if errorMessage.Valid {
		request.ErrorMessage = &errorMessage.String
	}
	return request, nil
}
