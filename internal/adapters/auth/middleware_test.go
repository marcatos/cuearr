package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/auth"
)

const testAPIKey = "test-api-key-secret"

func testMiddleware(t *testing.T, passwordHash, apiKey string) http.Handler {
	t.Helper()
	secret := []byte("test-session-secret")
	mw := auth.NewMiddleware(auth.MiddlewareConfig{
		SessionSecret: secret,
		GetAuth: func() (string, string) {
			return passwordHash, apiKey
		},
	})
	return mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
}

func TestMiddleware_Unauthorized(t *testing.T) {
	handler := testMiddleware(t, "$2a$10$hashed", testAPIKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_HealthPublic(t *testing.T) {
	handler := testMiddleware(t, "$2a$10$hashed", testAPIKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMiddleware_LoginPublic(t *testing.T) {
	handler := testMiddleware(t, "$2a$10$hashed", testAPIKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMiddleware_APIKeyOK(t *testing.T) {
	handler := testMiddleware(t, "$2a$10$hashed", testAPIKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set("X-Api-Key", testAPIKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_SessionOK(t *testing.T) {
	secret := []byte("test-session-secret")
	cookie, err := auth.NewSessionCookie(secret, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := testMiddleware(t, "$2a$10$hashed", testAPIKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_BootstrapSettingsAuthFromLocalhost(t *testing.T) {
	handler := testMiddleware(t, "", testAPIKey)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auth", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_BootstrapSettingsAuthDeniedWhenPasswordSet(t *testing.T) {
	handler := testMiddleware(t, "$2a$10$hashed", testAPIKey)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auth", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMiddleware_BootstrapSettingsAuthDeniedFromRemote(t *testing.T) {
	handler := testMiddleware(t, "", testAPIKey)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auth", nil)
	req.RemoteAddr = "203.0.113.50:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
