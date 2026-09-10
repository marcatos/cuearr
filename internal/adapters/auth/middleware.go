package auth

import (
	"net"
	"net/http"
	"strings"
)

// MiddlewareConfig configures API/session auth for /api routes.
type MiddlewareConfig struct {
	SessionSecret []byte
	GetAuth       func() (passwordHash, apiKey string)
}

// Middleware enforces auth on /api/* except public routes and bootstrap paths.
type Middleware struct {
	cfg MiddlewareConfig
}

func NewMiddleware(cfg MiddlewareConfig) *Middleware {
	return &Middleware{cfg: cfg}
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicAPI(r) {
			next.ServeHTTP(w, r)
			return
		}
		hash, apiKey := m.cfg.GetAuth()
		if hash == "" && isBootstrapSettingsAuth(r) {
			next.ServeHTTP(w, r)
			return
		}
		if ValidAPIKey(r.Header.Get("X-Api-Key"), apiKey) {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(SessionCookieName); err == nil && ValidSessionCookie(m.cfg.SessionSecret, c) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
}

func isPublicAPI(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/v1/health":
		return r.Method == http.MethodGet
	case "/api/v1/login":
		return true
	case "/api/v1/logout":
		return r.Method == http.MethodPost
	case "/api/v1/auth/oidc/login":
		return r.Method == http.MethodGet
	case "/api/v1/auth/oidc/callback":
		return r.Method == http.MethodGet
	default:
		return false
	}
}

func isBootstrapSettingsAuth(r *http.Request) bool {
	if r.Method != http.MethodPut || r.URL.Path != "/api/v1/settings/auth" {
		return false
	}
	return isLocalhost(r)
}

func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsLoginStaticPath reports paths served without API auth (embedded /login UI).
func IsLoginStaticPath(path string) bool {
	return path == "/login" || strings.HasPrefix(path, "/login/")
}
