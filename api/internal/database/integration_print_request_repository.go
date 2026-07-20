package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fly-print-cloud/api/internal/models"
)

var ErrIntegrationOrderConflict = errors.New("integration external order parameters conflict")
var ErrIntegrationTicketUnavailable = errors.New("terminal ticket is not selected for this provider")

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
	request.status,request.error_code,request.error_message,request.expires_at,request.created_at,request.updated_at`
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

func (r *IntegrationPrintRequestRepository) ClaimWaitingTerminal(now time.Time, lease time.Duration) (*models.IntegrationPrintRequest, error) {
	query := `WITH candidate AS (
		SELECT id FROM integration_print_requests WHERE status='waiting_terminal' AND expires_at>$1
		AND (worker_lease_until IS NULL OR worker_lease_until<$1) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE integration_print_requests AS request SET worker_lease_until=$2 FROM candidate WHERE request.id=candidate.id
	RETURNING request.id,request.provider_code,request.external_order_id,request.request_hash,request.terminal_ticket_hash,request.node_id,request.printer_id,request.terminal_session_id,
	request.external_user_id,request.external_user_name,request.file_url,request.file_name,request.file_size,request.mime_type,request.file_sha256,request.file_expires_at,request.print_options,request.metadata,request.file_id,request.print_job_id,
	request.status,request.error_code,request.error_message,request.expires_at,request.created_at,request.updated_at`
	request, err := scanIntegrationRequest(r.db.QueryRow(query, now, now.Add(lease)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return request, err
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
	result, err := tx.Exec(`UPDATE integration_print_requests SET status='dispatched',print_job_id=$2::uuid,worker_lease_until=NULL
		WHERE id=$1 AND status='waiting_terminal'`, requestID, jobID)
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
	status,error_code,error_message,expires_at,created_at,updated_at FROM integration_print_requests`

func scanIntegrationRequest(scanner interface{ Scan(...any) error }) (*models.IntegrationPrintRequest, error) {
	request := &models.IntegrationPrintRequest{}
	var externalUserID, externalUserName sql.NullString
	var fileID, printJobID, errorCode, errorMessage sql.NullString
	err := scanner.Scan(&request.ID, &request.ProviderCode, &request.ExternalOrderID, &request.RequestHash, &request.TerminalTicketHash,
		&request.NodeID, &request.PrinterID, &request.TerminalSessionID, &externalUserID, &externalUserName, &request.FileURL,
		&request.FileName, &request.FileSize, &request.MimeType, &request.FileSHA256, &request.FileExpiresAt, &request.PrintOptions, &request.Metadata,
		&fileID, &printJobID, &request.Status, &errorCode, &errorMessage, &request.ExpiresAt, &request.CreatedAt, &request.UpdatedAt)
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
