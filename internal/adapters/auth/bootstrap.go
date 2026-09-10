package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"

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
	if !settings.Auth.OIDCEnabled {
		if envBool("CUEARR_OIDC_ENABLED") {
			settings.Auth.OIDCEnabled = true
			changed = true
		}
	}
	if settings.Auth.OIDCIssuer == "" {
		if v := os.Getenv("CUEARR_OIDC_ISSUER"); v != "" {
			settings.Auth.OIDCIssuer = v
			changed = true
		}
	}
	if settings.Auth.OIDCClientID == "" {
		if v := os.Getenv("CUEARR_OIDC_CLIENT_ID"); v != "" {
			settings.Auth.OIDCClientID = v
			changed = true
		}
	}
	if settings.Auth.OIDCClientSecret == "" {
		if v := os.Getenv("CUEARR_OIDC_CLIENT_SECRET"); v != "" {
			settings.Auth.OIDCClientSecret = v
			changed = true
		}
	}
	if settings.Auth.OIDCRedirectURL == "" {
		if v := os.Getenv("CUEARR_OIDC_REDIRECT_URL"); v != "" {
			settings.Auth.OIDCRedirectURL = v
			changed = true
		}
	}
	if len(settings.Auth.OIDCAllowedEmailDomains) == 0 {
		if v := os.Getenv("CUEARR_OIDC_ALLOWED_EMAIL_DOMAINS"); v != "" {
			settings.Auth.OIDCAllowedEmailDomains = splitEnvCSV(v)
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

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitEnvCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// NewSessionSecret returns a random 32-byte session signing key.
func NewSessionSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("session secret: %w", err)
	}
	return secret, nil
}
