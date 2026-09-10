package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marcatos/cuearr/internal/domain"
	"golang.org/x/oauth2"
)

const (
	oidcStateCookieName = "cuearr_oidc_state"
	oidcStateTTL        = 10 * time.Minute
)

// OIDCConfig is the runtime OIDC configuration for login/callback.
type OIDCConfig struct {
	Enabled             bool
	Issuer              string
	ClientID            string
	ClientSecret        string
	RedirectURL         string
	AllowedEmailDomains []string
}

// OIDCConfigFromAuth maps persisted auth settings to OIDCConfig.
func OIDCConfigFromAuth(a domain.AuthSettings) OIDCConfig {
	return OIDCConfig{
		Enabled:             a.OIDCEnabled,
		Issuer:              a.OIDCIssuer,
		ClientID:            a.OIDCClientID,
		ClientSecret:        a.OIDCClientSecret,
		RedirectURL:         a.OIDCRedirectURL,
		AllowedEmailDomains: a.OIDCAllowedEmailDomains,
	}
}

func (c OIDCConfig) ready() bool {
	return c.Enabled && c.Issuer != "" && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

// EmailDomainAllowed reports whether email is permitted by allowed (empty = any domain).
func EmailDomainAllowed(email string, allowed []string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domainPart := email[at+1:]
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if strings.EqualFold(strings.TrimSpace(a), domainPart) {
			return true
		}
	}
	return false
}

// OIDCProviderDiscoverer loads an OIDC provider document (overridable in tests).
type OIDCProviderDiscoverer func(ctx context.Context, issuer string) (*oidc.Provider, error)

// OIDCHandler serves OIDC authorization-code login and callback.
type OIDCHandler struct {
	sessionSecret []byte
	sessionTTL    time.Duration
	getConfig     func(context.Context) (OIDCConfig, error)
	discover      OIDCProviderDiscoverer
}

// OIDCHandlerConfig configures OIDCHandler.
type OIDCHandlerConfig struct {
	SessionSecret []byte
	SessionTTL    time.Duration
	GetConfig     func(context.Context) (OIDCConfig, error)
	Discover      OIDCProviderDiscoverer
}

// NewOIDCHandler builds an OIDC login handler.
func NewOIDCHandler(cfg OIDCHandlerConfig) *OIDCHandler {
	discover := cfg.Discover
	if discover == nil {
		discover = oidc.NewProvider
	}
	ttl := cfg.SessionTTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &OIDCHandler{
		sessionSecret: cfg.SessionSecret,
		sessionTTL:    ttl,
		getConfig:     cfg.GetConfig,
		discover:      discover,
	}
}

// HandleLogin starts the authorization code flow (GET /api/v1/auth/oidc/login).
func (h *OIDCHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOIDCJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := h.getConfig(r.Context())
	if err != nil {
		slog.Error("oidc login: load config", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !cfg.ready() {
		writeOIDCJSONError(w, http.StatusNotFound, "oidc disabled")
		return
	}
	state, err := randomState()
	if err != nil {
		slog.Error("oidc login: state", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	oauthCfg, err := h.oauth2Config(r.Context(), cfg)
	if err != nil {
		slog.Error("oidc login: provider", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "oidc provider error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    state,
		Path:     "/api/v1/auth/oidc",
		MaxAge:   int(oidcStateTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	http.Redirect(w, r, oauthCfg.AuthCodeURL(state), http.StatusFound)
}

// HandleCallback completes the authorization code flow (GET /api/v1/auth/oidc/callback).
func (h *OIDCHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOIDCJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := h.getConfig(r.Context())
	if err != nil {
		slog.Error("oidc callback: load config", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !cfg.ready() {
		writeOIDCJSONError(w, http.StatusNotFound, "oidc disabled")
		return
	}
	stateCookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || stateCookie.Value == "" {
		writeOIDCJSONError(w, http.StatusBadRequest, "invalid state")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/api/v1/auth/oidc",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	qState := r.URL.Query().Get("state")
	if qState == "" || qState != stateCookie.Value {
		writeOIDCJSONError(w, http.StatusBadRequest, "invalid state")
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		slog.Warn("oidc callback: provider error", "error", errMsg)
		writeOIDCJSONError(w, http.StatusUnauthorized, "oidc authorization denied")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeOIDCJSONError(w, http.StatusBadRequest, "missing code")
		return
	}
	oauthCfg, err := h.oauth2Config(r.Context(), cfg)
	if err != nil {
		slog.Error("oidc callback: provider", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "oidc provider error")
		return
	}
	token, err := oauthCfg.Exchange(r.Context(), code)
	if err != nil {
		slog.Warn("oidc callback: token exchange", "error", err.Error())
		writeOIDCJSONError(w, http.StatusUnauthorized, "oidc token exchange failed")
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok || rawID == "" {
		writeOIDCJSONError(w, http.StatusUnauthorized, "missing id_token")
		return
	}
	provider, err := h.discover(r.Context(), cfg.Issuer)
	if err != nil {
		slog.Error("oidc callback: discover", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "oidc provider error")
		return
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(r.Context(), rawID)
	if err != nil {
		slog.Warn("oidc callback: verify id_token", "error", err.Error())
		writeOIDCJSONError(w, http.StatusUnauthorized, "invalid id_token")
		return
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		slog.Warn("oidc callback: parse claims", "error", err.Error())
		writeOIDCJSONError(w, http.StatusUnauthorized, "invalid claims")
		return
	}
	if claims.Email == "" {
		writeOIDCJSONError(w, http.StatusUnauthorized, "email claim required")
		return
	}
	if !EmailDomainAllowed(claims.Email, cfg.AllowedEmailDomains) {
		slog.Warn("oidc callback: email domain denied")
		writeOIDCJSONError(w, http.StatusForbidden, "email domain not allowed")
		return
	}
	session, err := NewSessionCookie(h.sessionSecret, h.sessionTTL)
	if err != nil {
		slog.Error("oidc callback: session", "error", err.Error())
		writeOIDCJSONError(w, http.StatusInternalServerError, "session error")
		return
	}
	http.SetCookie(w, session)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *OIDCHandler) oauth2Config(ctx context.Context, cfg OIDCConfig) (*oauth2.Config, error) {
	provider, err := h.discover(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover issuer: %w", err)
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}, nil
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func writeOIDCJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
