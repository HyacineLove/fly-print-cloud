package integration

// IsHTTPOrHTTPSScheme reports whether scheme is http or https (case-sensitive as from url.Parse).
func IsHTTPOrHTTPSScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}
