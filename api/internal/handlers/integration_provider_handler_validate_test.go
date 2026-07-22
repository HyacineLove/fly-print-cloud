package handlers

import (
	"strings"
	"testing"

	"fly-print-cloud/api/internal/models"
)

func TestIntegrationProviderValidateAcceptsHTTPAndHTTPS(t *testing.T) {
	handler := &IntegrationProviderHandler{}
	base := models.IntegrationProvider{
		DisplayName: "ACME",
		EntryURL:    "https://third.example.com/entry",
		CallbackBaseURL: "https://third.example.com",
	}
	if err := handler.validate(&base); err != nil {
		t.Fatalf("https provider URLs should be accepted: %v", err)
	}
	base.EntryURL = "http://192.168.1.10:8012/integration-demo/entry"
	base.CallbackBaseURL = "http://integration-demo:8080"
	if err := handler.validate(&base); err != nil {
		t.Fatalf("http provider URLs should be accepted: %v", err)
	}
}

func TestIntegrationProviderValidateRejectsNonHTTPSchemesAndUserinfo(t *testing.T) {
	handler := &IntegrationProviderHandler{}
	cases := []models.IntegrationProvider{
		{DisplayName: "x", EntryURL: "ftp://third.example.com/entry", CallbackBaseURL: "https://third.example.com"},
		{DisplayName: "x", EntryURL: "https://third.example.com/entry", CallbackBaseURL: "ws://third.example.com"},
		{DisplayName: "x", EntryURL: "https://user:pass@third.example.com/entry", CallbackBaseURL: "https://third.example.com"},
	}
	for _, provider := range cases {
		err := handler.validate(&provider)
		if err == nil || !strings.Contains(err.Error(), "HTTP or HTTPS") {
			t.Fatalf("expected HTTP or HTTPS validation error for %+v, got %v", provider, err)
		}
	}
}
