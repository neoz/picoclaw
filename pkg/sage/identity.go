package sage

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// IdentityManager manages per-user Ed25519 keypairs for Sage authentication.
// Keys are stored at {keysDir}/{owner}.key. owner="" maps to "_shared.key".
// The Sage agent ID is the hex-encoded public key (derived from the private key).
type IdentityManager struct {
	keysDir string
	client  *Client
	mu      sync.RWMutex
	cache   map[string]ed25519.PrivateKey // owner -> private key
}

// NewIdentityManager creates a new identity manager.
func NewIdentityManager(keysDir string, client *Client) *IdentityManager {
	return &IdentityManager{
		keysDir: keysDir,
		client:  client,
		cache:   make(map[string]ed25519.PrivateKey),
	}
}

// GetOrCreate returns the private key and hex-encoded public key (agent ID)
// for the given owner, creating and registering a new keypair if none exists.
func (im *IdentityManager) GetOrCreate(owner string) (ed25519.PrivateKey, string, error) {
	// Check cache first
	im.mu.RLock()
	if key, ok := im.cache[owner]; ok {
		im.mu.RUnlock()
		return key, pubKeyHex(key), nil
	}
	im.mu.RUnlock()

	im.mu.Lock()
	defer im.mu.Unlock()

	// Double-check after acquiring write lock
	if key, ok := im.cache[owner]; ok {
		return key, pubKeyHex(key), nil
	}

	// Try loading from disk
	keyPath := im.keyPath(owner)
	if data, err := os.ReadFile(keyPath); err == nil && len(data) == ed25519.PrivateKeySize {
		privKey := ed25519.PrivateKey(data)
		im.cache[owner] = privKey
		return privKey, pubKeyHex(privKey), nil
	}

	// Generate new keypair
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, "", fmt.Errorf("generate keypair: %w", err)
	}

	// Persist to disk
	if err := os.MkdirAll(im.keysDir, 0700); err != nil {
		return nil, "", fmt.Errorf("create keys dir: %w", err)
	}
	if err := os.WriteFile(keyPath, []byte(privKey), 0600); err != nil {
		return nil, "", fmt.Errorf("write key: %w", err)
	}

	agentID := pubKeyHex(privKey)

	// Register with Sage server
	name := "picoclaw"
	if owner != "" {
		name = "picoclaw_" + owner
	}
	if err := im.client.Register(agentID, privKey, RegisterRequest{
		AgentID:   agentID,
		Name:      name,
		PublicKey: agentID,
	}); err != nil {
		logger.WarnCF("sage", "Failed to register agent, will retry on next use",
			map[string]interface{}{"agent_id": agentID, "error": err.Error()})
	}

	im.cache[owner] = privKey
	return privKey, agentID, nil
}

func (im *IdentityManager) keyPath(owner string) string {
	filename := owner
	if filename == "" {
		filename = "_shared"
	}
	return filepath.Join(im.keysDir, filename+".key")
}

// pubKeyHex returns the hex-encoded public key from a private key.
func pubKeyHex(privKey ed25519.PrivateKey) string {
	return hex.EncodeToString(privKey.Public().(ed25519.PublicKey))
}
