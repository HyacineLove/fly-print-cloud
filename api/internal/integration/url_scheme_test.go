package integration

import "testing"

func TestIsHTTPOrHTTPSScheme(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		if !IsHTTPOrHTTPSScheme(scheme) {
			t.Fatalf("IsHTTPOrHTTPSScheme(%q) = false", scheme)
		}
	}
	for _, scheme := range []string{"", "ftp", "HTTP", "HTTPS", "ws", "wss", "file"} {
		if IsHTTPOrHTTPSScheme(scheme) {
			t.Fatalf("IsHTTPOrHTTPSScheme(%q) = true", scheme)
		}
	}
}
