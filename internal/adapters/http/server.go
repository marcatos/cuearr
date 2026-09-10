package httpapi

import (
	"context"
	"net/http"

	"github.com/marcatos/cuearr/internal/adapters/auth"
)

type Server struct {
	deps Deps
	mux  *http.ServeMux
}

func New(deps Deps) *Server {
	if deps.CheckSQLite == nil && deps.Jobs != nil {
		deps.CheckSQLite = func(ctx context.Context) error {
			_, err := deps.Jobs.List(ctx, 1)
			return err
		}
	}
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	secret := s.deps.SessionSecret
	mw := auth.NewMiddleware(auth.MiddlewareConfig{
		SessionSecret: secret,
		GetAuth: func() (string, string) {
			settings, err := s.deps.Settings.Get(context.Background())
			if err != nil {
				return "", ""
			}
			return settings.Auth.PasswordHash, settings.Auth.APIKey
		},
	})
	return mw.Wrap(s.mux)
}

func (s *Server) oidcHandler() *auth.OIDCHandler {
	return auth.NewOIDCHandler(auth.OIDCHandlerConfig{
		SessionSecret: s.deps.SessionSecret,
		CookieSecure:  s.deps.CookieSecure,
		GetConfig: func(ctx context.Context) (auth.OIDCConfig, error) {
			settings, err := s.deps.Settings.Get(ctx)
			if err != nil {
				return auth.OIDCConfig{}, err
			}
			return auth.OIDCConfigFromAuth(settings.Auth), nil
		},
	})
}

func (s *Server) routes() {
	oidc := s.oidcHandler()
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/auth/oidc/login", oidc.HandleLogin)
	s.mux.HandleFunc("GET /api/v1/auth/oidc/callback", oidc.HandleCallback)
	s.mux.HandleFunc("PUT /api/v1/settings/auth", s.handlePutSettingsAuth)
	s.mux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	s.mux.HandleFunc("POST /api/v1/jobs", s.handleCreateJob)
	s.mux.HandleFunc("POST /api/v1/jobs/scan", s.handleScanJobs)
	s.mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("GET /api/v1/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/v1/settings", s.handlePutSettings)
	s.mux.HandleFunc("POST /api/v1/hooks/lidarr", s.handleLidarrHook)

	static := newStaticHandler()
	s.mux.Handle("GET /{$}", static)
	s.mux.Handle("GET /login", static)
	s.mux.Handle("GET /login/{$}", static)
	s.mux.Handle("GET /app.js", static)
	s.mux.Handle("GET /styles.css", static)
	s.mux.Handle("GET /favicon.svg", static)
}
