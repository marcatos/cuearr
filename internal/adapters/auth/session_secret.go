package auth

import (
	"fmt"
	"os"
	"path/filepath"
)

const sessionKeyFile = "session.key"

const sessionSecretLen = 32

// LoadOrCreateSessionSecret reads the HMAC signing key from dataDir/session.key,
// or generates and persists one on first use.
func LoadOrCreateSessionSecret(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, sessionKeyFile)
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != sessionSecretLen {
			return nil, fmt.Errorf("session secret %s: want %d bytes, got %d", path, sessionSecretLen, len(data))
		}
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read session secret: %w", err)
	}
	secret, err := NewSessionSecret()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		return nil, fmt.Errorf("write session secret: %w", err)
	}
	return secret, nil
}
