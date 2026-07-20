package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const SignatureWindow = 300 * time.Second

// CanonicalString is shared by inbound verification and outbound callback
// signing. PATH must be the request URL path, not a configurable alias.
func CanonicalString(method, path, timestamp, nonce string, rawBody []byte) string {
	bodyHash := sha256.Sum256(rawBody)
	return strings.Join([]string{method, path, timestamp, nonce, fmt.Sprintf("%x", bodyHash)}, "\n")
}

func VerifySignature(secret, signature, method, path, timestamp, nonce string, rawBody []byte, now time.Time) error {
	if secret == "" || signature == "" || timestamp == "" || nonce == "" {
		return fmt.Errorf("required HMAC header is missing")
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || now.Sub(time.Unix(seconds, 0)).Abs() > SignatureWindow {
		return fmt.Errorf("HMAC timestamp is outside the allowed window")
	}
	received, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid HMAC signature encoding")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(CanonicalString(method, path, timestamp, nonce, rawBody)))
	if !hmac.Equal(received, mac.Sum(nil)) {
		return fmt.Errorf("HMAC signature does not match")
	}
	return nil
}

func Sign(secret, method, path, timestamp, nonce string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(CanonicalString(method, path, timestamp, nonce, rawBody)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
