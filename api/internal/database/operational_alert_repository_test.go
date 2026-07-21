package database

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOperationalAlertRepositorySummaryReturnsZeroCounts(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	query := `SELECT
		(SELECT COUNT(*) FROM operational_alerts WHERE status='open'),
		(SELECT COUNT(*) FROM operational_alerts WHERE status='open'),
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND enabled=true AND status='offline'),
		(SELECT COUNT(*) FROM printers p JOIN edge_nodes n ON n.id=p.edge_node_id
		 WHERE p.deleted_at IS NULL AND p.enabled=true AND n.enabled=true AND
		 n.status<>'offline' AND (p.status_received_at IS NULL OR
		  p.status_received_at < CURRENT_TIMESTAMP - INTERVAL '90 seconds' OR
		  p.status NOT IN ('idle','printing')))`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(
		sqlmock.NewRows([]string{
			"total",
			"high",
			"offline_nodes",
			"unavailable_printers",
		}).AddRow(0, 0, 0, 0),
	)

	repo := NewOperationalAlertRepository(&DB{db})
	summary, err := repo.Summary()
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary != (OperationalAlertSummary{}) {
		t.Fatalf("Summary() = %+v, want all zero counts", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestDeviceOverviewDoesNotCountOfflineNodePrintersAsFaults(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND enabled=true
			AND registration_state <> 'pending_activation'
			AND status='online' AND health_status IN ('degraded','critical')),
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND enabled=true
			AND registration_state <> 'pending_activation' AND status='online'),
		(SELECT COUNT(*) FROM edge_nodes WHERE deleted_at IS NULL AND registration_state <> 'pending_activation'),
		(SELECT COUNT(*) FROM printers p JOIN edge_nodes n ON n.id=p.edge_node_id
			WHERE p.deleted_at IS NULL AND p.enabled=true AND n.deleted_at IS NULL
				AND n.enabled=true AND n.status <> 'offline' AND (
				p.status_received_at IS NULL OR
				p.status_received_at < CURRENT_TIMESTAMP - INTERVAL '90 seconds' OR
				p.status NOT IN ('idle','printing')
			)),
		(SELECT COUNT(*) FROM printers p JOIN edge_nodes n ON n.id=p.edge_node_id
			WHERE p.deleted_at IS NULL AND p.enabled=true AND n.deleted_at IS NULL AND n.enabled=true
				AND n.status='online' AND p.status IN ('idle','printing')
				AND p.status_received_at IS NOT NULL
				AND p.status_received_at >= CURRENT_TIMESTAMP - INTERVAL '90 seconds'),
		(SELECT COUNT(*) FROM printers WHERE deleted_at IS NULL)`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{
		"fault_nodes", "online_nodes", "total_nodes", "fault_printers", "online_printers", "total_printers",
	}).AddRow(0, 0, 1, 0, 0, 1))

	overview, err := NewOperationalAlertRepository(&DB{db}).DeviceOverview()
	if err != nil {
		t.Fatalf("DeviceOverview() error = %v", err)
	}
	if overview.FaultPrinters != 0 || overview.TotalPrinters != 1 {
		t.Fatalf("unexpected overview: %+v", overview)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
