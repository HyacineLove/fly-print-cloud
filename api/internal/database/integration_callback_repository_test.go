package database

import "testing"

func TestIntegrationTransitionAllowedIsMonotonic(t *testing.T) {
	if !integrationTransitionAllowed("waiting_terminal", "dispatched") { t.Fatal("expected forward transition to be allowed") }
	if integrationTransitionAllowed("completed", "printing") { t.Fatal("completed state must not be reversed") }
	if integrationTransitionAllowed("failed", "completed") { t.Fatal("failed state must be terminal") }
	if integrationTransitionAllowed("printing", "waiting_terminal") { t.Fatal("backward transition must be rejected") }
}
