package database

import (
	"database/sql"
	"time"
)

type IntegrationCallbackEvent struct {
	ID            string
	EventID       string
	ProviderCode  string
	TargetURL     string
	Payload       string
	AttemptCount  int
	NextAttemptAt time.Time
}

type IntegrationCallbackRepository struct {
	db *DB
}

func NewIntegrationCallbackRepository(db *DB) *IntegrationCallbackRepository {
	return &IntegrationCallbackRepository{db: db}
}

// ValidateJobContext rejects late or cross-session Edge updates for integration
// jobs. Standard jobs have no integration row and retain their existing path.
func (r *IntegrationCallbackRepository) ValidateJobContext(jobID, sessionID, ticketHash, requestID string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM integration_print_requests WHERE print_job_id=$1::uuid`, jobID).Scan(&count)
	if err != nil || count == 0 {
		return err == nil, err
	}
	var matched bool
	err = r.db.QueryRow(`SELECT terminal_session_id=$2 AND terminal_ticket_hash=$3 AND id::text=$4 FROM integration_print_requests WHERE print_job_id=$1::uuid`, jobID, sessionID, ticketHash, requestID).Scan(&matched)
	return matched, err
}

// TransitionForJob advances only forward and creates its callback in the same
// integration transaction, so a status is never visible without an outbox row.
func (r *IntegrationCallbackRepository) TransitionForJob(jobID, status, errorCode, errorMessage string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var requestID, current string
	err = tx.QueryRow(`SELECT id,status FROM integration_print_requests
		WHERE print_job_id=$1::uuid FOR UPDATE`, jobID).Scan(&requestID, &current)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if !integrationTransitionAllowed(current, status) {
		return nil
	}
	if _, err = tx.Exec(`UPDATE integration_print_requests SET status=$2,error_code=NULLIF($3,''),error_message=NULLIF($4,'') WHERE id=$1::uuid`, requestID, status, errorCode, errorMessage); err != nil {
		return err
	}
	if err := enqueueIntegrationCallbackTx(tx, requestID, status, errorCode, errorMessage); err != nil {
		return err
	}
	return tx.Commit()
}
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func integrationTransitionAllowed(current, next string) bool {
	order := map[string]int{
		"accepted": 1, "waiting_file": 2, "waiting_terminal": 3,
		"dispatched": 4, "printing": 5, "completed": 6,
		"failed": 6, "expired": 6, "cancelled": 6,
	}
	if current == "completed" || current == "failed" || current == "expired" || current == "cancelled" {
		return false
	}
	return order[next] >= order[current]
}

// ClaimDue atomically leases one callback event. The temporary five-minute
// lease prevents a worker crash from leaving an event permanently in flight.
func (r *IntegrationCallbackRepository) ClaimDue(now time.Time) (*IntegrationCallbackEvent, error) {
	event := &IntegrationCallbackEvent{}
	err := r.db.QueryRow(`WITH candidate AS (
		SELECT id FROM integration_callback_events
		WHERE status='pending' AND next_attempt_at<=$1
		ORDER BY next_attempt_at FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE integration_callback_events event
	SET attempt_count=attempt_count+1,next_attempt_at=$2
	FROM candidate WHERE event.id=candidate.id
	RETURNING event.id,event.event_id,event.provider_code,event.target_url,event.payload::text,event.attempt_count,event.next_attempt_at`,
		now, now.Add(5*time.Minute),
	).Scan(&event.ID, &event.EventID, &event.ProviderCode, &event.TargetURL, &event.Payload, &event.AttemptCount, &event.NextAttemptAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return event, err
}

func (r *IntegrationCallbackRepository) Complete(id string) error {
	_, err := r.db.Exec(`UPDATE integration_callback_events SET status='delivered',delivered_at=CURRENT_TIMESTAMP WHERE id=$1::uuid`, id)
	return err
}

func (r *IntegrationCallbackRepository) Retry(id string, attempt int) error {
	if attempt >= 3 {
		tx, err := r.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`UPDATE integration_callback_events SET status='failed',last_error='delivery_failed' WHERE id=$1::uuid`, id); err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO operational_alerts(resource_type,resource_id,reason_code,category,title,details)
			SELECT 'integration_callback',event_id::text,'integration_callback_delivery_failed','integration',
			'Third-party callback delivery failed',jsonb_build_object('provider',provider_code,'event_id',event_id)
			FROM integration_callback_events WHERE id=$1::uuid
			ON CONFLICT (resource_type,resource_id,reason_code) WHERE status='open'
			DO UPDATE SET last_seen_at=CURRENT_TIMESTAMP,occurrence_count=operational_alerts.occurrence_count+1`, id)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	delay := time.Minute * time.Duration(1<<(attempt-1))
	if delay > 3*time.Hour {
		delay = 3 * time.Hour
	}
	_, err := r.db.Exec(`UPDATE integration_callback_events SET status='pending',next_attempt_at=$2,last_error='delivery_failed' WHERE id=$1::uuid`, id, time.Now().Add(delay))
	return err
}
