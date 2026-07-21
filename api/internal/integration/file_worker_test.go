package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"fly-print-cloud/api/internal/models"
)

func TestIsPublicAddressRejectsPrivateAndLoopbackRanges(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.1.1", "::1", "fe80::1"} {
		if isPublicAddress(net.ParseIP(value)) {
			t.Fatalf("isPublicAddress(%s) = true", value)
		}
	}
	if !isPublicAddress(net.ParseIP("8.8.8.8")) {
		t.Fatal("isPublicAddress() rejected a public address")
	}
	if !isAllowedProviderAddress(net.ParseIP("172.20.0.8"), true) {
		t.Fatal("explicit private provider policy should allow Docker private address")
	}
	if isAllowedProviderAddress(net.ParseIP("127.0.0.1"), true) || isAllowedProviderAddress(net.ParseIP("169.254.1.1"), true) {
		t.Fatal("private policy must not allow loopback or link-local addresses")
	}
}

func TestAllowedCSVContainsIsCaseInsensitiveAndExact(t *testing.T) {
	if !allowedCSVContains("files.example.com, cdn.example.com", "CDN.EXAMPLE.COM") {
		t.Fatal("allowedCSVContains() did not match configured host")
	}
	if allowedCSVContains("files.example.com", "files.example.com.evil.test") {
		t.Fatal("allowedCSVContains() matched a suffix host")
	}
}

func TestDownloadAndStoreRejectsExpiredProviderURLBeforeNetwork(t *testing.T) {
	worker := &FileWorker{}
	request := &models.IntegrationPrintRequest{
		FileURL:       "https://files.example.com/document.pdf",
		FileExpiresAt: time.Now().Add(-time.Second),
	}
	provider := &models.IntegrationProvider{AllowedFileHosts: "files.example.com"}
	if _, _, err := worker.downloadAndStore(context.Background(), request, provider); err == nil {
		t.Fatal("expired provider file URL must be rejected before any network request")
	}
}
