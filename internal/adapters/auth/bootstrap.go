package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

// Bootstrap applies env-based initial auth settings when none exist yet.
func Bootstrap(ctx context.Context, store ports.SettingsStore) (domain.Settings, bool, error) {
	settings, err := store.Get(ctx)
	if err != nil {
		return domain.Settings{}, false, fmt.Errorf("get settings: %w", err)
	}
	changed := false
	if settings.Auth.PasswordHash == "" {
		if pw := os.Getenv("CUEARR_INITIAL_PASSWORD"); pw != "" {
			hash, err := HashPassword(pw)
			if err != nil {
				return domain.Settings{}, false, err
			}
			settings.Auth.PasswordHash = hash
			changed = true
		}
	}
	if settings.Auth.APIKey == "" {
		if key := os.Getenv("CUEARR_API_KEY"); key != "" {
			settings.Auth.APIKey = key
			changed = true
		}
	}
	if changed {
		if err := store.Put(ctx, settings); err != nil {
			return domain.Settings{}, false, fmt.Errorf("put settings: %w", err)
		}
	}
	return settings, changed, nil
}

// NewSessionSecret returns a random 32-byte session signing key.
func NewSessionSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("session secret: %w", err)
	}
	return secret, nil
}
