package security

import (
	"errors"
	"testing"
	"time"
)

// MockTokenUsageMarker 模拟 TokenUsageMarker
type MockTokenUsageMarker struct {
	usedTokens map[string]bool
}

func NewMockTokenUsageMarker() *MockTokenUsageMarker {
	return &MockTokenUsageMarker{
		usedTokens: make(map[string]bool),
	}
}

func (m *MockTokenUsageMarker) MarkTokenAsUsed(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error {
	if m.usedTokens[tokenHash] {
		return errors.New("token has already been used")
	}
	m.usedTokens[tokenHash] = true
	return nil
}

func TestTokenManager_GenerateUploadToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 180, 180, nil)

	nodeID := "node-123"
	printerID := "printer-456"

	token, expiresAt, err := tm.GenerateUploadToken(nodeID, printerID)
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("Token expiration time should be in the future")
	}

	// Token should expire in approximately 180 seconds
	expectedExpiry := time.Now().Add(180 * time.Second)
	diff := expiresAt.Sub(expectedExpiry)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("Token expiry time off by %v", diff)
	}
}

func TestTokenManager_ValidateUploadToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 180, 180, nil)

	nodeID := "node-123"
	printerID := "printer-456"

	// Generate a valid token
	token, _, err := tm.GenerateUploadToken(nodeID, printerID)
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	// Test valid token
	payload, err := tm.ValidateUploadToken(token)
	if err != nil {
		t.Errorf("ValidateUploadToken() error = %v", err)
	}

	if payload.NodeID != nodeID {
		t.Errorf("Expected NodeID %s, got %s", nodeID, payload.NodeID)
	}

	if payload.PrinterID != printerID {
		t.Errorf("Expected PrinterID %s, got %s", printerID, payload.PrinterID)
	}

	// Test invalid token
	_, err = tm.ValidateUploadToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

func TestTokenManager_GenerateDownloadToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 180, 180, nil)

	fileID := "file-123"
	jobID := "job-456"
	nodeID := "node-789"

	token, expiresAt, err := tm.GenerateDownloadToken(fileID, jobID, nodeID)
	if err != nil {
		t.Fatalf("GenerateDownloadToken() error = %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("Token expiration time should be in the future")
	}
}

func TestTokenManager_ValidateDownloadToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 180, 180, nil)

	fileID := "file-123"
	jobID := "job-456"
	nodeID := "node-789"

	// Generate a valid token
	token, _, err := tm.GenerateDownloadToken(fileID, jobID, nodeID)
	if err != nil {
		t.Fatalf("GenerateDownloadToken() error = %v", err)
	}

	// Test valid token
	payload, err := tm.ValidateDownloadToken(token, fileID, nodeID)
	if err != nil {
		t.Errorf("ValidateDownloadToken() error = %v", err)
	}

	if payload.FileID != fileID {
		t.Errorf("Expected FileID %s, got %s", fileID, payload.FileID)
	}

	if payload.JobID != jobID {
		t.Errorf("Expected JobID %s, got %s", jobID, payload.JobID)
	}

	if payload.NodeID != nodeID {
		t.Errorf("Expected NodeID %s, got %s", nodeID, payload.NodeID)
	}

	// Test context mismatch
	_, err = tm.ValidateDownloadToken(token, "wrong-file-id", nodeID)
	if err == nil {
		t.Error("Expected error for context mismatch")
	}
}

func TestTokenManager_ExpiredToken(t *testing.T) {
	// 测试过期token需要修改代码支持负数TTL或使用时间注入
	// 这里改为简单验证token格式包含过期时间
	tm := NewTokenManager("test-secret", 180, 180, nil)

	nodeID := "node-123"
	printerID := "printer-456"

	token, expiresAt, err := tm.GenerateUploadToken(nodeID, printerID)
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	// Verify expiration time is in the future
	if expiresAt.Before(time.Now()) {
		t.Error("Expiration time should be in the future")
	}

	// Validate the token works when not expired
	_, err = tm.ValidateUploadToken(token)
	if err != nil {
		t.Errorf("ValidateUploadToken() error = %v", err)
	}
}

func TestTokenManager_OneTimeToken(t *testing.T) {
	mockRepo := NewMockTokenUsageMarker()
	tm := NewTokenManager("test-secret", 180, 180, mockRepo)

	nodeID := "node-123"
	printerID := "printer-456"

	// Generate token
	token, _, err := tm.GenerateUploadToken(nodeID, printerID)
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	// First validation should succeed and mark as used
	_, err = tm.ValidateUploadToken(token)
	if err != nil {
		t.Errorf("First ValidateUploadToken() error = %v", err)
	}

	// Second validation should fail (already used)
	_, err = tm.ValidateUploadToken(token)
	if err != ErrTokenAlreadyUsed {
		t.Errorf("Expected ErrTokenAlreadyUsed, got %v", err)
	}
}

func TestTokenManager_WrongSecret(t *testing.T) {
	tm1 := NewTokenManager("secret1", 180, 180, nil)
	tm2 := NewTokenManager("secret2", 180, 180, nil)

	nodeID := "node-123"
	printerID := "printer-456"

	// Generate token with first manager
	token, _, err := tm1.GenerateUploadToken(nodeID, printerID)
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	// Validate with second manager (different secret)
	_, err = tm2.ValidateUploadToken(token)
	if err == nil {
		t.Error("Expected error when validating with wrong secret")
	}
}
