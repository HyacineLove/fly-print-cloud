package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"fly-print-cloud/api/internal/models"
	"github.com/lib/pq"
)

// AlertSpec is the Cloud-owned, stable operational meaning of a reason code.
type AlertSpec struct {
	Category string
	Title    string
}

type OperationalAlertRepository struct{ db *DB }

type OperationalAlertSummary struct {
	Total               int `json:"total"`
	High                int `json:"high"`
	OfflineNodes        int `json:"offline_nodes"`
	UnavailablePrinters int `json:"unavailable_printers"`
}

// DeviceOverview is intentionally separate from alert counts: a connected
// node can still be faulty, and the Admin UI must be able to show both facts.
type DeviceOverview struct {
	FaultNodes     int `json:"fault_nodes"`
	OnlineNodes    int `json:"online_nodes"`
	TotalNodes     int `json:"total_nodes"`
	FaultPrinters  int `json:"fault_printers"`
	OnlinePrinters int `json:"online_printers"`
	TotalPrinters  int `json:"total_printers"`
}

func NewOperationalAlertRepository(db *DB) *OperationalAlertRepository {
	return &OperationalAlertRepository{db: db}
}

func (r *OperationalAlertRepository) OpenTx(tx *Tx, resourceType, resourceID, nodeID, printerID, jobID, reasonCode string, spec AlertSpec, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal alert details: %w", err)
	}
	_, err = tx.Exec(`
		INSERT INTO operational_alerts (
			resource_type, resource_id, node_id, printer_id, job_id, reason_code,
			category, title, details
		) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, '')::uuid, NULLIF($5, '')::uuid,
			$6, $7, $8, $9)
		ON CONFLICT (resource_type, resource_id, reason_code) WHERE status = 'open'
		DO UPDATE SET last_seen_at = CURRENT_TIMESTAMP,
			title = EXCLUDED.title,
			details = EXCLUDED.details, updated_at = CURRENT_TIMESTAMP`,
		resourceType, resourceID, nodeID, printerID, jobID, reasonCode,
		spec.Category, spec.Title, detailsJSON,
	)
	return err
}

func (r *OperationalAlertRepository) ResolvePrinterReasonsByNodeTx(tx *Tx, nodeID string, reasons []string) error {
	if len(reasons) == 0 {
		return nil
	}
	_, err := tx.Exec(`UPDATE operational_alerts SET status='resolved', resolved_at=CURRENT_TIMESTAMP,
		updated_at=CURRENT_TIMESTAMP WHERE resource_type='printer' AND node_id=$1 AND status='open'
		AND reason_code = ANY($2)`, nodeID, pq.Array(reasons))
	return err
}

func (r *OperationalAlertRepository) ResolveOtherTx(tx *Tx, resourceType, resourceID string, activeReasons []string) error {
	if len(activeReasons) == 0 {
		_, err := tx.Exec(`UPDATE operational_alerts SET status='resolved', resolved_at=CURRENT_TIMESTAMP,
			updated_at=CURRENT_TIMESTAMP WHERE resource_type=$1 AND resource_id=$2 AND status='open'`, resourceType, resourceID)
		return err
	}
	_, err := tx.Exec(`UPDATE operational_alerts SET status='resolved', resolved_at=CURRENT_TIMESTAMP,
		updated_at=CURRENT_TIMESTAMP WHERE resource_type=$1 AND resource_id=$2 AND status='open'
		AND NOT (reason_code = ANY($3))`, resourceType, resourceID, pq.Array(activeReasons))
	return err
}

func (r *OperationalAlertRepository) ResolveReasonsTx(tx *Tx, resourceType, resourceID string, managedReasons, activeReasons []string) error {
	if len(managedReasons) == 0 {
		return nil
	}
	_, err := tx.Exec(`UPDATE operational_alerts SET status='resolved', resolved_at=CURRENT_TIMESTAMP,
		updated_at=CURRENT_TIMESTAMP WHERE resource_type=$1 AND resource_id=$2 AND status='open'
		AND reason_code = ANY($3) AND NOT (reason_code = ANY($4))`, resourceType, resourceID,
		pq.Array(managedReasons), pq.Array(activeReasons))
	return err
}

func (r *OperationalAlertRepository) List(status, resourceType, nodeID, printerID string, from, to *time.Time, offset, limit int) ([]*models.OperationalAlert, int, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	add := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		where += fmt.Sprintf(" AND %s = $%d", column, len(args))
	}
	add("a.status", status)
	add("a.resource_type", resourceType)
	add("a.node_id", nodeID)
	add("a.printer_id::text", printerID)
	if from != nil {
		args = append(args, *from)
		where += fmt.Sprintf(" AND a.first_seen_at >= $%d", len(args))
	}
	if to != nil {
		args = append(args, *to)
		where += fmt.Sprintf(" AND a.first_seen_at <= $%d", len(args))
	}
	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM operational_alerts a"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.db.Query(`SELECT a.id::text, a.resource_type, a.resource_id,
		COALESCE(a.node_id,''), COALESCE(a.printer_id::text,''), COALESCE(a.job_id::text,''),
		a.reason_code, a.category, a.title, a.status,
		a.details, a.occurrence_count, a.first_seen_at, a.last_seen_at, a.resolved_at,
		COALESCE(n.alias, n.name, ''), COALESCE(p.display_name, p.name, ''), COALESCE(j.name, '')
		FROM operational_alerts a LEFT JOIN edge_nodes n ON n.id=a.node_id
		LEFT JOIN printers p ON p.id=a.printer_id
		LEFT JOIN print_jobs j ON j.id=a.job_id`+where+fmt.Sprintf(" ORDER BY a.last_seen_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	alerts := []*models.OperationalAlert{}
	for rows.Next() {
		a := &models.OperationalAlert{}
		var details []byte
		var resolved sql.NullTime
		if err := rows.Scan(&a.ID, &a.ResourceType, &a.ResourceID, &a.NodeID, &a.PrinterID, &a.JobID,
			&a.ReasonCode, &a.Category, &a.Title, &a.Status,
			&details, &a.OccurrenceCount, &a.FirstSeenAt, &a.LastSeenAt, &resolved, &a.NodeName, &a.PrinterName, &a.JobName); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(details, &a.Details)
		if resolved.Valid {
			a.ResolvedAt = &resolved.Time
		}
		if a.ResolvedAt != nil {
			a.DurationSeconds = int64(a.ResolvedAt.Sub(a.FirstSeenAt).Seconds())
		}
		alerts = append(alerts, a)
	}
	return alerts, total, rows.Err()
}

func (r *OperationalAlertRepository) DeviceOverview() (DeviceOverview, error) {
	var overview DeviceOverview
	err := r.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND enabled=true
			AND registration_state <> 'pending_activation'
			AND status='online' AND health_status IN ('degraded','critical')),
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND enabled=true
			AND registration_state <> 'pending_activation' AND status='online'),
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND registration_state <> 'pending_activation'),
		(SELECT COUNT(*) FROM printers p JOIN edge_nodes n ON n.id=p.edge_node_id
			WHERE p.deleted_at IS NULL AND p.enabled=true AND n.deleted_at IS NULL AND (
				NOT n.enabled OR n.status <> 'online' OR p.status_received_at IS NULL OR
				p.status_received_at < CURRENT_TIMESTAMP - INTERVAL '90 seconds' OR
				p.status NOT IN ('idle','printing')
			)),
		(SELECT COUNT(*) FROM printers p JOIN edge_nodes n ON n.id=p.edge_node_id
			WHERE p.deleted_at IS NULL AND p.enabled=true AND n.deleted_at IS NULL AND n.enabled=true
				AND n.status='online' AND p.status IN ('idle','printing')
				AND p.status_received_at IS NOT NULL
				AND p.status_received_at >= CURRENT_TIMESTAMP - INTERVAL '90 seconds'),
		(SELECT COUNT(*) FROM printers WHERE deleted_at IS NULL)`).Scan(
		&overview.FaultNodes, &overview.OnlineNodes, &overview.TotalNodes,
		&overview.FaultPrinters, &overview.OnlinePrinters, &overview.TotalPrinters,
	)
	return overview, err
}

func (r *OperationalAlertRepository) Summary() (OperationalAlertSummary, error) {
	var summary OperationalAlertSummary
	err := r.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM operational_alerts WHERE status='open'),
		(SELECT COUNT(*) FROM operational_alerts WHERE status='open'),
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND enabled=true AND status='offline'),
		(SELECT COUNT(*) FROM printers p JOIN edge_nodes n ON n.id=p.edge_node_id
		 WHERE p.deleted_at IS NULL AND p.enabled=true AND n.enabled=true AND
		 (n.status<>'online' OR p.status_received_at IS NULL OR
		  p.status_received_at < CURRENT_TIMESTAMP - INTERVAL '90 seconds' OR
		  p.status NOT IN ('idle','printing')))`).Scan(
		&summary.Total, &summary.High, &summary.OfflineNodes, &summary.UnavailablePrinters,
	)
	return summary, err
}

func (r *OperationalAlertRepository) CleanupResolved(retention time.Duration) (int64, error) {
	result, err := r.db.Exec(`DELETE FROM operational_alerts WHERE status='resolved'
		AND resolved_at < CURRENT_TIMESTAMP - ($1 * INTERVAL '1 second')`, int64(retention.Seconds()))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteForNodeTx removes alerts owned by a node and its printers before the
// node's soft-delete and printer cascade are committed.
func (r *OperationalAlertRepository) DeleteForNodeTx(tx *Tx, nodeID string) error {
	_, err := tx.Exec(`DELETE FROM operational_alerts
		WHERE node_id = $1
		   OR (resource_type = 'node' AND resource_id = $1)
		   OR printer_id IN (SELECT id FROM printers WHERE edge_node_id = $1)`, nodeID)
	return err
}

func (r *OperationalAlertRepository) DeleteForPrinter(printerID string) error {
	_, err := r.db.Exec(`DELETE FROM operational_alerts
		WHERE printer_id = $1::uuid
		   OR (resource_type = 'printer' AND resource_id = $1)`, printerID)
	return err
}

func (r *OperationalAlertRepository) DeleteForJob(jobID string) error {
	_, err := r.db.Exec(`DELETE FROM operational_alerts
		WHERE job_id = $1::uuid
		   OR (resource_type = 'job' AND resource_id = $1)`, jobID)
	return err
}

// CleanupOrphaned removes alerts whose owning resource was deleted. Older
// deployments did not have foreign-key cascades for operational alerts, so
// this also repairs orphan rows already present in the database.
func (r *OperationalAlertRepository) CleanupOrphaned() (int64, error) {
	result, err := r.db.Exec(`DELETE FROM operational_alerts a
		WHERE (a.node_id IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM edge_nodes n WHERE n.id = a.node_id AND n.deleted_at IS NULL
		))
		OR (a.printer_id IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM printers p WHERE p.id = a.printer_id AND p.deleted_at IS NULL
		))
		OR (a.job_id IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM print_jobs j WHERE j.id = a.job_id
		))
		OR (a.resource_type = 'node' AND NOT EXISTS (
			SELECT 1 FROM edge_nodes n WHERE n.id::text = a.resource_id AND n.deleted_at IS NULL
		))
		OR (a.resource_type = 'printer' AND NOT EXISTS (
			SELECT 1 FROM printers p WHERE p.id::text = a.resource_id AND p.deleted_at IS NULL
		))
		OR (a.resource_type = 'job' AND NOT EXISTS (
			SELECT 1 FROM print_jobs j WHERE j.id::text = a.resource_id
		))`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
