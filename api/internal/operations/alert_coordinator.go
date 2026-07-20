package operations

import (
	"time"

	"fly-print-cloud/api/internal/database"
)

type AlertCoordinator struct {
	repository *database.OperationalAlertRepository
}

func NewAlertCoordinator(repository *database.OperationalAlertRepository) *AlertCoordinator {
	return &AlertCoordinator{repository: repository}
}

func (c *AlertCoordinator) ReconcilePrinter(tx *database.Tx, resourceID, nodeID, printerStatus string, statusSince time.Time, enabled bool, now time.Time) error {
	active := []string{}
	if policy, ok := alertPolicy(printerStatus); ok && printerStatus != "" && enabled {
		active = append(active, printerStatus)
		if policyReady(policy, statusSince, now) {
			if err := c.repository.OpenTx(tx, "printer", resourceID, nodeID, resourceID, "", printerStatus, policy.AlertSpec, nil); err != nil {
				return err
			}
		}
	}
	return c.repository.ResolveOtherTx(tx, "printer", resourceID, active)
}

func (c *AlertCoordinator) OpenPrinterUnconfirmed(tx *database.Tx, printerID, nodeID, jobID, errorCode string, details map[string]interface{}) error {
	policy, _ := alertPolicy("printer_unconfirmed_lock")
	if details == nil {
		details = map[string]interface{}{}
	}
	details["job_id"] = jobID
	details["error_code"] = errorCode
	return c.repository.OpenTx(tx, "printer", printerID, nodeID, printerID, jobID, "printer_unconfirmed_lock", policy.AlertSpec, details)
}

func (c *AlertCoordinator) SuppressPrinterConnections(tx *database.Tx, nodeID string) error {
	return c.repository.ResolvePrinterReasonsByNodeTx(tx, nodeID, connectionScopedPrinterReasons())
}
