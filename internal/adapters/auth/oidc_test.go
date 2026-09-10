package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marcatos/cuearr/internal/adapters/auth"
)

func TestEmailDomainAllowed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		email   string
		allowed []string
		want    bool
	}{
		{"alice@example.com", nil, true},
		{"alice@example.com", []string{}, true},
		{"alice@example.com", []string{"example.com"}, true},
		{"alice@example.com", []string{"EXAMPLE.COM"}, true},
		{"alice@example.com", []string{"other.com"}, false},
		{"not-an-email", []string{"example.com"}, false},
		{"@example.com", []string{"example.com"}, false},
	}
	for _, tt := range tests {
		if got := auth.EmailDomainAllowed(tt.email, tt.allowed); got != tt.want {
			t.Errorf("EmailDomainAllowed(%q, %v) = %v, want %v", tt.email, tt.allowed, got, tt.want)
		}
	}
}

func TestOIDCHandler_Login_Disabled(t *testing.T) {
	h := auth.NewOIDCHandler(auth.OIDCHandlerConfig{
		SessionSecret: []byte("test-session-secret"),
		SessionTTL:    time.Hour,
		GetConfig: func(context.Context) (auth.OIDCConfig, error) {
			return auth.OIDCConfig{Enabled: false}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rec := httptest.NewRecorder()
	h.HandleLogin(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOIDCHandler_Callback_Disabled(t *testing.T) {
	h := auth.NewOIDCHandler(auth.OIDCHandlerConfig{
		SessionSecret: []byte("test-session-secret"),
		SessionTTL:    time.Hour,
		GetConfig: func(context.Context) (auth.OIDCConfig, error) {
			return auth.OIDCConfig{Enabled: false}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=x&state=y", nil)
	rec := httptest.NewRecorder()
	h.HandleCallback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOIDCHandler_Callback_InvalidStateWhenEnabled(t *testing.T) {
	h := auth.NewOIDCHandler(auth.OIDCHandlerConfig{
		SessionSecret: []byte("test-session-secret"),
		SessionTTL:    time.Hour,
		GetConfig: func(context.Context) (auth.OIDCConfig, error) {
			return auth.OIDCConfig{
				Enabled:      true,
				Issuer:       "https://issuer.example",
				ClientID:     "client",
				ClientSecret: "secret",
				RedirectURL:  "http://localhost/callback",
			}, nil
		},
		Discover: func(context.Context, string) (*oidc.Provider, error) {
			t.Fatal("discover should not run without valid state")
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=x&state=y", nil)
	rec := httptest.NewRecorder()
	h.HandleCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
