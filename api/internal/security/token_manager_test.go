package security

import (
	"fmt"
	"testing"
	"time"
)

type testTokenRecord struct {
	tokenType  string
	nodeID     string
	resourceID string
	jobID      string
	expiresAt  time.Time
	revoked    bool
	used       bool
}

type testTokenRepo struct {
	records map[string]*testTokenRecord
}

func newTestTokenRepo() *testTokenRepo {
	return &testTokenRepo{records: map[string]*testTokenRecord{}}
}

func (r *testTokenRepo) PreRegisterToken(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error {
	if _, ok := r.records[tokenHash]; ok {
		return nil
	}
	r.records[tokenHash] = &testTokenRecord{
		tokenType:  tokenType,
		nodeID:     nodeID,
		resourceID: resourceID,
		jobID:      jobID,
		expiresAt:  expiresAt,
	}
	return nil
}

func (r *testTokenRepo) MarkTokenAsUsed(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error {
	if existing, ok := r.records[tokenHash]; ok {
		if existing.revoked {
			return fmt.Errorf("token has been revoked")
		}
		if existing.used {
			return fmt.Errorf("token has already been used")
		}
		existing.used = true
		return nil
	}

	r.records[tokenHash] = &testTokenRecord{
		tokenType:  tokenType,
		nodeID:     nodeID,
		resourceID: resourceID,
		jobID:      jobID,
		expiresAt:  expiresAt,
		used:       true,
	}
	return nil
}

func (r *testTokenRepo) RevokeTokensByNodeAndResource(tokenType, nodeID, resourceID string) (int64, error) {
	var revoked int64
	for _, record := range r.records {
		if record.tokenType == tokenType && record.nodeID == nodeID && record.resourceID == resourceID && !record.revoked {
			record.revoked = true
			revoked++
		}
	}
	return revoked, nil
}

func (r *testTokenRepo) GetTokenStatus(tokenHash string) (bool, bool, bool, error) {
	record, ok := r.records[tokenHash]
	if !ok {
		return false, false, false, nil
	}
	return record.used, record.revoked, true, nil
}

func TestGenerateUploadTokenProducesUniqueTokensForRapidRefresh(t *testing.T) {
	repo := newTestTokenRepo()
	tm := NewTokenManager("secret", 180, 180, repo)

	first, _, err := tm.GenerateUploadToken("node-1", "printer-1")
	if err != nil {
		t.Fatalf("first token: %v", err)
	}

	second, _, err := tm.GenerateUploadToken("node-1", "printer-1")
	if err != nil {
		t.Fatalf("second token: %v", err)
	}

	if first == second {
		t.Fatalf("expected rapid refresh tokens to be unique")
	}

	if _, err := tm.ValidateUploadToken(second); err != nil {
		t.Fatalf("expected latest token to validate, got %v", err)
	}

	if _, err := tm.ValidateUploadToken(first); GetTokenErrorCode(err) != "token_revoked" {
		t.Fatalf("expected previous token to be revoked, got %v", err)
	}
}

func TestVerifyUploadTokenAvailableRejectsUsedToken(t *testing.T) {
	repo := newTestTokenRepo()
	tm := NewTokenManager("secret", 180, 180, repo)

	token, _, err := tm.GenerateUploadToken("node-1", "printer-1")
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	if _, err := tm.ValidateUploadToken(token); err != nil {
		t.Fatalf("ValidateUploadToken() error = %v", err)
	}

	if _, err := tm.VerifyUploadTokenAvailable(token, "node-1", "printer-1"); GetTokenErrorCode(err) != "token_already_used" {
		t.Fatalf("error code = %s, want token_already_used", GetTokenErrorCode(err))
	}
}

func TestVerifyUploadTokenAvailableRejectsWrongContext(t *testing.T) {
	repo := newTestTokenRepo()
	tm := NewTokenManager("secret", 180, 180, repo)

	token, _, err := tm.GenerateUploadToken("node-1", "printer-1")
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	if _, err := tm.VerifyUploadTokenAvailable(token, "node-2", "printer-1"); GetTokenErrorCode(err) != "invalid_context" {
		t.Fatalf("error code = %s, want invalid_context", GetTokenErrorCode(err))
	}
}
