package websocket

import "testing"

func TestBuildProxyFileURL(t *testing.T) {
	t.Parallel()

	got := buildProxyFileURL("file-123")
	if got != "/api/v1/files/file-123" {
		t.Fatalf("buildProxyFileURL() = %q, want %q", got, "/api/v1/files/file-123")
	}
}

func TestExtractProxyFileID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fileURL string
		want    string
	}{
		{name: "proxy path", fileURL: "/api/v1/files/file-123", want: "file-123"},
		{name: "proxy path with token", fileURL: "/api/v1/files/file-123?token=abc", want: "file-123"},
		{name: "absolute URL ignored in phase one", fileURL: "https://minio.local/bucket/object?X-Amz-Signature=abc", want: ""},
		{name: "empty", fileURL: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractProxyFileID(tt.fileURL); got != tt.want {
				t.Fatalf("extractProxyFileID(%q) = %q, want %q", tt.fileURL, got, tt.want)
			}
		})
	}
}
