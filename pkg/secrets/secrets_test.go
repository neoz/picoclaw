package secrets

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func tempKeyPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".secret_key")
}

func TestRoundtrip(t *testing.T) {
	store, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	original := "sk-or-v1-abc123"
	encrypted, err := store.Encrypt(original)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == original {
		t.Fatal("encrypted should differ from original")
	}
	if !IsEncrypted(encrypted) {
		t.Fatal("encrypted value should have enc: prefix")
	}

	decrypted, err := store.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != original {
		t.Fatalf("roundtrip failed: got %q, want %q", decrypted, original)
	}
}

func TestEmptyString(t *testing.T) {
	store, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := store.Encrypt("")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted != "" {
		t.Fatalf("empty string should remain empty, got %q", encrypted)
	}
}

func TestPlaintextPassthrough(t *testing.T) {
	store, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	plain := "sk-or-v1-abc123"
	result, err := store.Decrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if result != plain {
		t.Fatalf("plaintext passthrough failed: got %q, want %q", result, plain)
	}
}

func TestAlreadyEncryptedPassthrough(t *testing.T) {
	store, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := store.Encrypt("my-secret")
	if err != nil {
		t.Fatal(err)
	}

	// Encrypting again should return the same value
	doubleEncrypted, err := store.Encrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if doubleEncrypted != encrypted {
		t.Fatal("already-encrypted value should pass through unchanged")
	}
}

func TestTamperDetection(t *testing.T) {
	store, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := store.Encrypt("my-secret")
	if err != nil {
		t.Fatal(err)
	}

	// Flip a byte in the hex-encoded ciphertext
	raw := []byte(encrypted)
	if raw[len(raw)-1] == '0' {
		raw[len(raw)-1] = '1'
	} else {
		raw[len(raw)-1] = '0'
	}
	tampered := string(raw)

	_, err = store.Decrypt(tampered)
	if err == nil {
		t.Fatal("tampered ciphertext should fail decryption")
	}
}

func TestDifferentNonces(t *testing.T) {
	store, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	enc1, err := store.Encrypt("same-value")
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := store.Encrypt("same-value")
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Fatal("two encryptions of same value should produce different ciphertexts")
	}

	// Both should decrypt to the same value
	dec1, _ := store.Decrypt(enc1)
	dec2, _ := store.Decrypt(enc2)
	if dec1 != dec2 {
		t.Fatal("both ciphertexts should decrypt to same plaintext")
	}
}

func TestKeyGenerationAndPersistence(t *testing.T) {
	keyPath := tempKeyPath(t)

	store1, err := NewSecretStore(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := store1.Encrypt("persistent-test")
	if err != nil {
		t.Fatal(err)
	}

	// Load the same key from disk
	store2, err := NewSecretStore(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := store2.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "persistent-test" {
		t.Fatalf("key persistence failed: got %q", decrypted)
	}
}

func TestInvalidKeyFile(t *testing.T) {
	keyPath := tempKeyPath(t)
	os.WriteFile(keyPath, []byte("not-valid-hex!"), 0600)

	_, err := NewSecretStore(keyPath)
	if err == nil {
		t.Fatal("expected error for invalid key file")
	}
}

func TestWrongKeyDecryption(t *testing.T) {
	store1, err := NewSecretStore(tempKeyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := store1.Encrypt("secret-data")
	if err != nil {
		t.Fatal(err)
	}

	// Create a different store with a different key
	keyPath2 := tempKeyPath(t)
	// Write a different valid key
	differentKey := make([]byte, 32)
	differentKey[0] = 0xFF
	os.WriteFile(keyPath2, []byte(hex.EncodeToString(differentKey)), 0600)
	store2, err := NewSecretStore(keyPath2)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("decryption with wrong key should fail")
	}
}
