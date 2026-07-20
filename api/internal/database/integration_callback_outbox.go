package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// enqueueIntegrationCallbackTx records the provider-visible state in the same
// transaction that changed the request. It deliberately contains no delivery
// logic: the callback worker is the only component that performs HTTP I/O.
func enqueueIntegrationCallbackTx(tx *sql.Tx, requestID, status, errorCode, errorMessage string) error {
	var providerCode, orderID, callbackBase, terminalName string
	var jobID sql.NullString
	err := tx.QueryRow(`SELECT request.provider_code,request.external_order_id,provider.callback_base_url,
		request.print_job_id::text,COALESCE(printer.name,'')
		FROM integration_print_requests request
		JOIN integration_providers provider ON provider.code=request.provider_code
		LEFT JOIN printers printer ON printer.id=request.printer_id
		WHERE request.id=$1::uuid`, requestID).Scan(&providerCode, &orderID, &callbackBase, &jobID, &terminalName)
	if err != nil {
		return err
	}

	eventID := uuid.NewString()
	payload, err := json.Marshal(map[string]interface{}{
		"event_id":          eventID,
		"request_id":        requestID,
		"external_order_id": orderID,
		"flyprint_job_id":   nullableString(jobID),
		"status":            status,
		"error_code":        nullIfEmpty(errorCode),
		"error_message":     nullIfEmpty(errorMessage),
		"terminal_name":     terminalName,
		"updated_at":        time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	payloadHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	_, err = tx.Exec(`INSERT INTO integration_callback_events(event_id,request_id,provider_code,target_url,payload,payload_hash,status,next_attempt_at)
		VALUES($1,$2::uuid,$3,$4,$5::jsonb,$6,'pending',CURRENT_TIMESTAMP)`,
		eventID, requestID, providerCode, strings.TrimRight(callbackBase, "/")+"/api/print/callback", string(payload), payloadHash)
	return err
}

func nullableString(value sql.NullString) interface{} {
	if !value.Valid {
		return nil
	}
	return value.String
}
