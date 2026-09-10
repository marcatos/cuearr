package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/auth"
	"github.com/marcatos/cuearr/internal/domain"
)

type jobJSON struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	CuePath     string `json:"cue_path"`
	ImagePath   string `json:"image_path"`
	OutDir      string `json:"out_dir,omitempty"`
	Status      string `json:"status"`
	Engine      string `json:"engine"`
	Log         string `json:"log,omitempty"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"created_at"`
	StartedAt   string `json:"started_at,omitempty"`
	FinishedAt  string `json:"finished_at,omitempty"`
}

type authSettingsJSON struct {
	PasswordConfigured      bool     `json:"password_configured,omitempty"`
	APIKeySet               bool     `json:"api_key_set,omitempty"`
	OIDCClientSecretSet     bool     `json:"oidc_client_secret_set,omitempty"`
	Password                string   `json:"password,omitempty"`
	APIKey                  string   `json:"api_key,omitempty"`
	OIDCEnabled             bool     `json:"oidc_enabled,omitempty"`
	OIDCIssuer              string   `json:"oidc_issuer,omitempty"`
	OIDCClientID            string   `json:"oidc_client_id,omitempty"`
	OIDCClientSecret        string   `json:"oidc_client_secret,omitempty"`
	OIDCRedirectURL         string   `json:"oidc_redirect_url,omitempty"`
	OIDCAllowedEmailDomains []string `json:"oidc_allowed_email_domains,omitempty"`
}

type settingsJSON struct {
	WatchDirs []string         `json:"watch_dirs"`
	OutDir    string           `json:"out_dir"`
	InPlace   bool             `json:"in_place"`
	Engine    string           `json:"engine"`
	Auth      authSettingsJSON `json:"auth"`
}

type settingsPutJSON struct {
	WatchDirs []string          `json:"watch_dirs"`
	OutDir    string            `json:"out_dir"`
	InPlace   bool              `json:"in_place"`
	Engine    string            `json:"engine"`
	Auth      *authSettingsJSON `json:"auth"`
}

func jobToJSON(j domain.Job) jobJSON {
	out := jobJSON{
		ID:          j.ID,
		Fingerprint: j.Fingerprint,
		CuePath:     j.CuePath,
		ImagePath:   j.ImagePath,
		OutDir:      j.OutDir,
		Status:      j.Status,
		Engine:      j.Engine,
		Log:         j.Log,
		Error:       j.Error,
		CreatedAt:   formatTime(j.CreatedAt),
	}
	if !j.StartedAt.IsZero() {
		out.StartedAt = formatTime(j.StartedAt)
	}
	if !j.FinishedAt.IsZero() {
		out.FinishedAt = formatTime(j.FinishedAt)
	}
	return out
}

func settingsToJSON(s domain.Settings) settingsJSON {
	return settingsJSON{
		WatchDirs: s.WatchDirs,
		OutDir:    s.OutDir,
		InPlace:   s.InPlace,
		Engine:    s.Engine,
		Auth: authSettingsJSON{
			PasswordConfigured:      s.Auth.PasswordHash != "",
			APIKeySet:               s.Auth.APIKey != "",
			OIDCClientSecretSet:     s.Auth.OIDCClientSecret != "",
			OIDCEnabled:             s.Auth.OIDCEnabled,
			OIDCIssuer:              s.Auth.OIDCIssuer,
			OIDCClientID:            s.Auth.OIDCClientID,
			OIDCRedirectURL:         s.Auth.OIDCRedirectURL,
			OIDCAllowedEmailDomains: append([]string(nil), s.Auth.OIDCAllowedEmailDomains...),
		},
	}
}

func settingsFromJSON(in settingsJSON) domain.Settings {
	return domain.Settings{
		WatchDirs: in.WatchDirs,
		OutDir:    in.OutDir,
		InPlace:   in.InPlace,
		Engine:    in.Engine,
	}
}

func mergeAuthSettings(existing domain.AuthSettings, in authSettingsJSON) (domain.AuthSettings, error) {
	out := existing
	out.OIDCEnabled = in.OIDCEnabled
	out.OIDCIssuer = in.OIDCIssuer
	out.OIDCClientID = in.OIDCClientID
	out.OIDCRedirectURL = in.OIDCRedirectURL
	if in.OIDCAllowedEmailDomains != nil {
		out.OIDCAllowedEmailDomains = append([]string(nil), in.OIDCAllowedEmailDomains...)
	}
	if in.OIDCClientSecret != "" {
		out.OIDCClientSecret = in.OIDCClientSecret
	}
	if in.APIKey != "" {
		out.APIKey = in.APIKey
	}
	if in.Password != "" {
		hash, err := auth.HashPassword(in.Password)
		if err != nil {
			return domain.AuthSettings{}, err
		}
		out.PasswordHash = hash
	}
	return out, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "error", err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx := r.Context()
	sqliteOK := true
	if s.deps.CheckSQLite != nil {
		if err := s.deps.CheckSQLite(ctx); err != nil {
			sqliteOK = false
			slog.Warn("health sqlite check failed", "error", err.Error())
		}
	}
	shntoolOK := false
	shntoolChecked := false
	if s.deps.CheckShntool != nil {
		shntoolChecked = true
		if err := s.deps.CheckShntool(ctx); err == nil {
			shntoolOK = true
		}
	}
	resp := map[string]any{
		"ok":        sqliteOK,
		"sqlite_ok": sqliteOK,
	}
	if shntoolChecked {
		resp["shntool_ok"] = shntoolOK
	}
	status := http.StatusOK
	if !sqliteOK {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	jobs, err := s.deps.Jobs.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]jobJSON, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobToJSON(j))
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.deps.CreateJob == nil {
		writeError(w, http.StatusInternalServerError, "create job not configured")
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	job, created, err := s.deps.CreateJob(r.Context(), req.Path)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, domain.ErrImageNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	code := http.StatusCreated
	if !created {
		code = http.StatusOK
	}
	writeJSON(w, code, map[string]any{
		"job":     jobToJSON(job),
		"created": created,
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := r.PathValue("id")
	job, err := s.deps.Jobs.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobToJSON(job))
}

func (s *Server) handleScanJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.deps.ScanWatch == nil {
		writeError(w, http.StatusInternalServerError, "scan not configured")
		return
	}
	watchDirs := s.deps.WatchDirs
	if s.deps.RuntimeSettings != nil {
		watchDirs = s.deps.RuntimeSettings().WatchDirs
	}
	if len(watchDirs) == 0 {
		writeError(w, http.StatusBadRequest, "no watch_dirs configured")
		return
	}
	scan := s.deps.ScanWatch
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := scan(ctx); err != nil {
			slog.Error("background watch scan failed", "error", err.Error())
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan started"})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	settings, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings.Engine == "" && s.deps.Engine != "" {
		settings.Engine = s.deps.Engine
	}
	if len(settings.WatchDirs) == 0 && len(s.deps.WatchDirs) > 0 {
		settings.WatchDirs = s.deps.WatchDirs
	}
	writeJSON(w, http.StatusOK, settingsToJSON(settings))
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body settingsPutJSON
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	settings := domain.Settings{
		WatchDirs: body.WatchDirs,
		OutDir:    body.OutDir,
		InPlace:   body.InPlace,
		Engine:    body.Engine,
	}
	existing, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	settings.Auth = existing.Auth
	if body.Auth != nil {
		settings.Auth, err = mergeAuthSettings(existing.Auth, *body.Auth)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := s.deps.Settings.Put(r.Context(), settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.deps.ApplySettings != nil {
		if err := s.deps.ApplySettings(r.Context(), settings); err != nil {
			if rollbackErr := s.deps.Settings.Put(r.Context(), existing); rollbackErr != nil {
				slog.Error("settings rollback failed", "error", rollbackErr.Error())
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, settingsToJSON(settings))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	settings, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings.Auth.PasswordHash == "" {
		writeError(w, http.StatusUnauthorized, "password not configured")
		return
	}
	if !auth.CheckPassword(settings.Auth.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	cookie, err := auth.NewSessionCookie(s.deps.SessionSecret, 7*24*time.Hour, s.deps.CookieSecure)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session error")
		return
	}
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	http.SetCookie(w, auth.ClearSessionCookie(s.deps.CookieSecure))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePutSettingsAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	settings, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings.Auth.PasswordHash != "" {
		writeError(w, http.StatusForbidden, "auth already configured")
		return
	}
	var req struct {
		Password string `json:"password"`
		APIKey   string `json:"api_key,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	settings.Auth.PasswordHash = hash
	if req.APIKey != "" {
		settings.Auth.APIKey = req.APIKey
	}
	if err := s.deps.Settings.Put(r.Context(), settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
