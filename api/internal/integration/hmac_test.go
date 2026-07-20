package integration

import (
	"testing"
	"time"
)

func TestVerifySignatureUsesRawBodyAndConstantCanonicalFields(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"message":"原始内容"}`)
	signature := Sign("test-secret", "POST", "/api/v1/integrations/livacloud/print-requests", "1700000000", "nonce-1", body)
	if err := VerifySignature("test-secret", signature, "POST", "/api/v1/integrations/livacloud/print-requests", "1700000000", "nonce-1", body, now); err != nil {
		t.Fatalf("VerifySignature() error = %v", err)
	}
	if err := VerifySignature("test-secret", signature, "POST", "/api/v1/integrations/livacloud/print-requests", "1700000000", "nonce-1", []byte(`{"message":"tampered"}`), now); err == nil {
		t.Fatal("VerifySignature() accepted a changed raw body")
	}
}

func TestVerifySignatureRejectsOldTimestamp(t *testing.T) {
	body := []byte("{}")
	signature := Sign("test-secret", "POST", "/path", "1699999600", "nonce-1", body)
	if err := VerifySignature("test-secret", signature, "POST", "/path", "1699999600", "nonce-1", body, time.Unix(1_700_000_000, 1)); err == nil {
		t.Fatal("VerifySignature() accepted a timestamp outside the 300 second window")
	}
}
