package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestOIDCEmailAllowed_RequiresVerificationForDomainAllowList(t *testing.T) {
	t.Parallel()
	verified := true
	unverified := false
	tests := []struct {
		name     string
		verified *bool
		allowed  []string
		want     bool
	}{
		{name: "verified", verified: &verified, allowed: []string{"example.com"}, want: true},
		{name: "false", verified: &unverified, allowed: []string{"example.com"}, want: false},
		{name: "missing", verified: nil, allowed: []string{"example.com"}, want: false},
		{name: "allow list disabled", verified: nil, allowed: nil, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auth.OIDCEmailAllowed("alice@example.com", tt.verified, tt.allowed); got != tt.want {
				t.Fatalf("allowed=%v want %v", got, tt.want)
			}
		})
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

func TestOIDCHandler_Login_Misconfigured(t *testing.T) {
	h := auth.NewOIDCHandler(auth.OIDCHandlerConfig{
		SessionSecret: []byte("test-session-secret"),
		SessionTTL:    time.Hour,
		GetConfig: func(context.Context) (auth.OIDCConfig, error) {
			return auth.OIDCConfig{Enabled: true, Issuer: "https://issuer.example"}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rec := httptest.NewRecorder()
	h.HandleLogin(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "oidc misconfigured") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestOIDCHandler_Login_CachesProviderDiscovery(t *testing.T) {
	var discoveries atomic.Int32
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discoveries.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})

	h := auth.NewOIDCHandler(auth.OIDCHandlerConfig{
		SessionSecret: []byte("test-session-secret"),
		SessionTTL:    time.Hour,
		GetConfig: func(context.Context) (auth.OIDCConfig, error) {
			return auth.OIDCConfig{
				Enabled:      true,
				Issuer:       srv.URL,
				ClientID:     "client",
				ClientSecret: "secret",
				RedirectURL:  "http://localhost/callback",
			}, nil
		},
		Discover: oidc.NewProvider,
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
		rec := httptest.NewRecorder()
		h.HandleLogin(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("login %d status=%d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	if got := discoveries.Load(); got != 1 {
		t.Fatalf("discoveries=%d want 1", got)
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
