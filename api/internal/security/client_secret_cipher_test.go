package security

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testEncryptionKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestClientSecretCipherRoundTrip(t *testing.T) {
	cipher, err := NewClientSecretCipher(testEncryptionKey())
	if err != nil {
		t.Fatalf("NewClientSecretCipher() error = %v", err)
	}
	encrypted, err := cipher.Encrypt("edge-secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if strings.Contains(encrypted, "edge-secret") {
		t.Fatal("encrypted value contains plaintext")
	}
	plain, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plain != "edge-secret" {
		t.Fatalf("Decrypt() = %q, want edge-secret", plain)
	}
}

func TestClientSecretCipherUsesRandomNonce(t *testing.T) {
	cipher, _ := NewClientSecretCipher(testEncryptionKey())
	first, _ := cipher.Encrypt("same-secret")
	second, _ := cipher.Encrypt("same-secret")
	if first == second {
		t.Fatal("encrypting the same secret must produce distinct ciphertext")
	}
}

func TestClientSecretCipherRejectsMissingOrInvalidKey(t *testing.T) {
	for _, key := range []string{"", "not-base64", base64.StdEncoding.EncodeToString([]byte("too-short"))} {
		if _, err := NewClientSecretCipher(key); err == nil {
			t.Fatalf("NewClientSecretCipher(%q) error = nil", key)
		}
	}
}

func TestClientSecretCipherRejectsTamperedCiphertext(t *testing.T) {
	cipher, _ := NewClientSecretCipher(testEncryptionKey())
	encrypted, _ := cipher.Encrypt("edge-secret")
	last := encrypted[len(encrypted)-1]
	replacement := byte('A')
	if last == replacement {
		replacement = 'B'
	}
	tampered := encrypted[:len(encrypted)-1] + string(replacement)
	if _, err := cipher.Decrypt(tampered); err == nil {
		t.Fatal("Decrypt() accepted tampered ciphertext")
	}
}
