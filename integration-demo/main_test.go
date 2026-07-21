package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHMACUsesCanonicalBody(t *testing.T) {
	body := []byte(`{"status":"completed"}`)
	timestamp := fmtUnix(time.Now())
	signature := sign("secret", http.MethodPost, "/api/print/callback", timestamp, "nonce", body)
	if !verify("secret", signature, http.MethodPost, "/api/print/callback", timestamp, "nonce", body) {
		t.Fatal("valid signature rejected")
	}
	if verify("secret", signature, http.MethodPost, "/api/print/callback", timestamp, "nonce", append(body, ' ')) {
		t.Fatal("tampered body accepted")
	}
}

func TestSetupStoresSecretsWithoutReturningThem(t *testing.T) {
	s := &server{dataDir: t.TempDir(), adminPassword: "admin", state: persistedState{Orders: map[string]order{}, Events: map[string]bool{}}}
	body := []byte(`{"password":"admin","inbound_secret":"in-secret","outbound_secret":"out-secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.saveSetup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "in-secret") || strings.Contains(rec.Body.String(), "out-secret") {
		t.Fatal("setup response exposed a secret")
	}
	if !s.configured() {
		t.Fatal("setup was not persisted")
	}
}

func TestCallbackIsIdempotentByEventID(t *testing.T) {
	s := &server{dataDir: t.TempDir(), adminPassword: "admin", state: persistedState{Configuration: configuration{OutboundSecret: "callback-secret"}, Orders: map[string]order{"order-1": {ID: "order-1", Status: "waiting_terminal"}}, Events: map[string]bool{}}}
	payload, _ := json.Marshal(map[string]string{"event_id": "event-1", "external_order_id": "order-1", "status": "completed"})
	timestamp := fmtUnix(time.Now())
	signature := sign("callback-secret", http.MethodPost, "/api/print/callback", timestamp, "nonce", payload)
	call := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/print/callback", bytes.NewReader(payload))
		req.Header.Set("X-FP-Client", providerCode)
		req.Header.Set("X-FP-Timestamp", timestamp)
		req.Header.Set("X-FP-Nonce", "nonce")
		req.Header.Set("X-FP-Signature", signature)
		rec := httptest.NewRecorder()
		s.callback(rec, req)
		return rec.Code
	}
	if code := call(); code != http.StatusNoContent {
		t.Fatalf("first callback status=%d", code)
	}
	if code := call(); code != http.StatusNoContent {
		t.Fatalf("duplicate callback status=%d", code)
	}
	if s.state.Orders["order-1"].Status != "completed" || len(s.state.Events) != 1 {
		t.Fatal("callback was not applied idempotently")
	}
}

func TestEntryPreservesTicketInSessionStorage(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodGet, "/entry?terminal_ticket=opaque", nil)
	rec := httptest.NewRecorder()
	s.entry(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "sessionStorage.setItem('flyprint_terminal_ticket'") {
		t.Fatal("entry did not persist terminal ticket")
	}
}

func fmtUnix(value time.Time) string { return strconv.FormatInt(value.Unix(), 10) }
