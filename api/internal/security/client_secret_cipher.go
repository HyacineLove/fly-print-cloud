package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedSecretVersion = "v1"

type ClientSecretCipher struct {
	aead cipher.AEAD
}

func NewClientSecretCipher(encodedKey string) (*ClientSecretCipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("oauth client secret encryption key must be base64-encoded 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &ClientSecretCipher{aead: aead}, nil
}

func (c *ClientSecretCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, sealed...)
	return encryptedSecretVersion + "." + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *ClientSecretCipher) Decrypt(value string) (string, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 || parts[0] != encryptedSecretVersion {
		return "", errors.New("unsupported encrypted secret format")
	}
	payload, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(payload) <= c.aead.NonceSize() {
		return "", errors.New("invalid encrypted secret")
	}
	nonce, ciphertext := payload[:c.aead.NonceSize()], payload[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("decrypt client secret failed")
	}
	return string(plaintext), nil
}
