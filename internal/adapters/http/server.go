package httpapi

import (
	"context"
	"net/http"
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
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	s.mux.HandleFunc("POST /api/v1/jobs", s.handleCreateJob)
	s.mux.HandleFunc("POST /api/v1/jobs/scan", s.handleScanJobs)
	s.mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("GET /api/v1/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/v1/settings", s.handlePutSettings)
	s.mux.HandleFunc("POST /api/v1/hooks/lidarr", s.handleLidarrHook)
}
