package integration

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NonceStore provides the only replay store allowed for provider requests.
// Redis SET NX EX makes the replay decision atomic across Cloud instances.
type NonceStore struct {
	redisURL *url.URL
}

func NewNonceStore(rawURL string) (*NonceStore, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "redis" && parsedURL.Scheme != "rediss") || parsedURL.Host == "" {
		return nil, fmt.Errorf("integration Redis URL must use redis:// or rediss://")
	}
	return &NonceStore{redisURL: parsedURL}, nil
}

func (s *NonceStore) Use(ctx context.Context, provider, nonce string) (bool, error) {
	if provider == "" || nonce == "" {
		return false, fmt.Errorf("provider and nonce are required")
	}
	connection, err := s.dial(ctx)
	if err != nil {
		return false, err
	}
	defer connection.Close()

	reader := bufio.NewReader(connection)
	if password, present := s.redisURL.User.Password(); present {
		if err := writeRESP(connection, "AUTH", password); err != nil {
			return false, err
		}
		if err := expectOK(reader); err != nil {
			return false, err
		}
	}
	if databaseNumber := strings.TrimPrefix(s.redisURL.Path, "/"); databaseNumber != "" {
		if _, err := strconv.Atoi(databaseNumber); err != nil {
			return false, fmt.Errorf("invalid Redis database number")
		}
		if err := writeRESP(connection, "SELECT", databaseNumber); err != nil {
			return false, err
		}
		if err := expectOK(reader); err != nil {
			return false, err
		}
	}

	key := "flyprint:integration:nonce:" + provider + ":" + nonce
	if err := writeRESP(connection, "SET", key, "1", "NX", "EX", "300"); err != nil {
		return false, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read Redis nonce result: %w", err)
	}
	switch strings.TrimSpace(line) {
	case "+OK":
		return true, nil
	case "$-1":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected Redis nonce result")
	}
}

func (s *NonceStore) dial(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	}
	address := s.redisURL.Host
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(address, "6379")
	}
	if s.redisURL.Scheme == "rediss" {
		return tls.DialWithDialer(&dialer, "tcp", address, &tls.Config{ServerName: s.redisURL.Hostname(), MinVersion: tls.VersionTLS12})
	}
	return dialer.DialContext(ctx, "tcp", address)
}

func writeRESP(connection net.Conn, arguments ...string) error {
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(arguments)))
	builder.WriteString("\r\n")
	for _, argument := range arguments {
		builder.WriteString("$")
		builder.WriteString(strconv.Itoa(len(argument)))
		builder.WriteString("\r\n")
		builder.WriteString(argument)
		builder.WriteString("\r\n")
	}
	_, err := connection.Write([]byte(builder.String()))
	return err
}

func expectOK(reader *bufio.Reader) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "+OK" {
		return fmt.Errorf("Redis command rejected")
	}
	return nil
}
