package operations

import (
	"testing"
	"time"

	"fly-print-cloud/api/internal/models"
)

func TestValidatePrinterDispatchUsesOrderedFacts(t *testing.T) {
	now := time.Now()
	fresh := now.Add(-10 * time.Second)
	node := &models.EdgeNode{Enabled: true, ConnectionStatus: "online"}
	printer := &models.Printer{Enabled: true, PrinterStatus: "idle", StatusReceivedAt: &fresh}

	if got := ValidatePrinterDispatch(printer, node, now); got != "" {
		t.Fatalf("idle printer should accept a task: got %q", got)
	}
	printer.PrinterStatus = "printing"
	if got := ValidatePrinterDispatch(printer, node, now); got != "printer_busy" {
		t.Fatalf("processing printer: got %q", got)
	}
	printer.PrinterStatus = "printer_out_of_paper"
	if got := ValidatePrinterDispatch(printer, node, now); got != "printer_out_of_paper" {
		t.Fatalf("fault must be returned directly: got %q", got)
	}
	printer.PrinterStatus = "idle"
	old := now.Add(-91 * time.Second)
	printer.StatusReceivedAt = &old
	if got := ValidatePrinterDispatch(printer, node, now); got != "printer_status_stale" || !PrinterStatusStale(printer, now) {
		t.Fatalf("stale printer: got %q", got)
	}
	printer.StatusReceivedAt = &fresh
	node.ConnectionStatus = "unstable"
	if got := ValidatePrinterDispatch(printer, node, now); got != "node_offline" {
		t.Fatalf("unstable node: got %q", got)
	}
	node.ConnectionStatus = "offline"
	if got := ValidatePrinterDispatch(printer, node, now); got != "node_offline" {
		t.Fatalf("offline node: got %q", got)
	}
}

func TestAlertPolicyActivationDelays(t *testing.T) {
	now := time.Now()
	tests := []struct {
		reason string
		age    time.Duration
		ready  bool
	}{
		{"printer_out_of_paper", 0, true},
		{"printer_not_accepting_jobs", 59 * time.Second, false},
		{"printer_not_accepting_jobs", 60 * time.Second, true},
	}
	for _, test := range tests {
		policy, ok := alertPolicy(test.reason)
		if !ok {
			t.Fatalf("missing policy for %s", test.reason)
		}
		if got := policyReady(policy, now.Add(-test.age), now); got != test.ready {
			t.Fatalf("%s age=%s: got ready=%v", test.reason, test.age, got)
		}
	}
}

func TestAttentionReasonsDoNotCreatePolicies(t *testing.T) {
	for _, reason := range []string{"printer_warning", "printer_toner_low", "node_connection_unstable", "libreoffice_unavailable"} {
		if _, ok := alertPolicy(reason); ok {
			t.Fatalf("attention reason %q must not create an operational alert", reason)
		}
	}
}

func TestNormalPrinterStatusesDoNotCreateAlerts(t *testing.T) {
	for _, status := range []string{"idle", "printing"} {
		if _, ok := alertPolicy(status); ok {
			t.Fatalf("normal printer status %q must not be an alert reason", status)
		}
	}
}

func TestConnectionScopedReasonsOnlyContainConnectivityFailures(t *testing.T) {
	reasons := map[string]bool{}
	for _, reason := range connectionScopedPrinterReasons() {
		reasons[reason] = true
	}
	if !reasons["ipp_unreachable"] {
		t.Fatal("IPP connectivity failures must be suppressible under node offline")
	}
	if _, ok := alertPolicy("printer_offline"); ok {
		t.Fatal("offline state must not be treated as a printer alert")
	}
	if _, ok := alertPolicy("printer_state_unknown"); ok {
		t.Fatal("unknown state must not be treated as a printer alert")
	}
	if reasons["printer_out_of_paper"] || reasons["printer_jammed"] {
		t.Fatal("physical faults must remain visible under node offline")
	}
}

func TestOfflineStatesDoNotCreateAlertPolicies(t *testing.T) {
	for _, reason := range []string{"node_offline", "printer_offline", "printer_state_unknown"} {
		if _, ok := alertPolicy(reason); ok {
			t.Fatalf("offline state %q must not create an operational alert", reason)
		}
	}
}

func TestApplyJobResultRejectsUnknownStatusBeforeDatabaseWrite(t *testing.T) {
	service := &StatusService{}
	if err := service.ApplyJobResult("job", "node", "printer", "printing", "", nil); err == nil {
		t.Fatal("legacy printing status must not enter the Cloud state model")
	}
}
