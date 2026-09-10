package httpapi

import (
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/auth"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestMergeAuthSettings_PreservesExistingWhenPartialUpdate(t *testing.T) {
	existing := domain.AuthSettings{
		PasswordHash: "$2a$10$existing",
		APIKey:       "keep-me",
		OIDCEnabled:  false,
	}
	merged, err := mergeAuthSettings(existing, authSettingsJSON{
		OIDCEnabled: true,
		APIKey:      "new-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if merged.PasswordHash != existing.PasswordHash {
		t.Fatalf("password hash changed: %q", merged.PasswordHash)
	}
	if merged.APIKey != "new-key" {
		t.Fatalf("api_key=%q", merged.APIKey)
	}
	if !merged.OIDCEnabled {
		t.Fatal("expected oidc_enabled=true")
	}
}

func TestMergeAuthSettings_SetsPasswordHash(t *testing.T) {
	merged, err := mergeAuthSettings(domain.AuthSettings{}, authSettingsJSON{
		Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if merged.PasswordHash == "" {
		t.Fatal("expected password hash")
	}
	if !auth.CheckPassword(merged.PasswordHash, "secret") {
		t.Fatal("password hash does not verify")
	}
}
