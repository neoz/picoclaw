package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
)

const encPrefix = "enc:"

// SecretStore handles encryption and decryption of sensitive config values
// using ChaCha20-Poly1305 AEAD.
type SecretStore struct {
	key [32]byte
}

// NewSecretStore loads an existing key or generates a new one at keyPath.
// Key file contains 64 hex characters (32 bytes) with 0600 permissions.
func NewSecretStore(keyPath string) (*SecretStore, error) {
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("secrets: create key directory: %w", err)
	}

	data, err := os.ReadFile(keyPath)
	if err == nil {
		return loadKey(strings.TrimSpace(string(data)))
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("secrets: read key file: %w", err)
	}

	// Generate new key
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, fmt.Errorf("secrets: generate key: %w", err)
	}

	encoded := hex.EncodeToString(key[:])
	if err := os.WriteFile(keyPath, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("secrets: write key file: %w", err)
	}

	return &SecretStore{key: key}, nil
}

func loadKey(hexKey string) (*SecretStore, error) {
	decoded, err := hex.DecodeString(hexKey)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("secrets: invalid key file (expected 64 hex characters)")
	}
	s := &SecretStore{}
	copy(s.key[:], decoded)
	return s, nil
}

// Encrypt returns "enc:" + hex(nonce || ciphertext || tag).
// Empty strings and already-encrypted values are returned unchanged.
func (s *SecretStore) Encrypt(plaintext string) (string, error) {
	if plaintext == "" || strings.HasPrefix(plaintext, encPrefix) {
		return plaintext, nil
	}

	aead, err := chacha20poly1305.NewX(s.key[:])
	if err != nil {
		return "", fmt.Errorf("secrets: create cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + hex.EncodeToString(ciphertext), nil
}

// Decrypt strips the "enc:" prefix, hex-decodes, and decrypts.
// Plaintext values (no "enc:" prefix) are returned unchanged.
func (s *SecretStore) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, encPrefix) {
		return ciphertext, nil
	}

	raw, err := hex.DecodeString(ciphertext[len(encPrefix):])
	if err != nil {
		return "", fmt.Errorf("secrets: hex decode: %w", err)
	}

	aead, err := chacha20poly1305.NewX(s.key[:])
	if err != nil {
		return "", fmt.Errorf("secrets: create cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("secrets: ciphertext too short")
	}

	plaintext, err := aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("secrets: decrypt: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted returns true if the value has the "enc:" prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encPrefix)
}
