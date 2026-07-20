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
		 (n.status<>'online' OR p.status_received_at IS NULL OR
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
