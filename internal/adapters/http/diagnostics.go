package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/marcatos/cuearr/internal/domain"
)

const (
	diagnosticsJobLimit = 20
	diagnosticsLogLimit = 2 * 1024
)

type diagnosticsAuthJSON struct {
	PasswordConfigured bool `json:"password_configured"`
	APIKeySet          bool `json:"api_key_set"`
	OIDCEnabled        bool `json:"oidc_enabled"`
}

type diagnosticsToolJSON struct {
	Available bool   `json:"available"`
	Error     string `json:"error"`
}

type diagnosticsJobJSON struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	AlbumLabel   string `json:"album_label"`
	CueFile      string `json:"cue_file"`
	ImageFile    string `json:"image_file"`
	Log          string `json:"log"`
	Error        string `json:"error"`
	AttemptCount int    `json:"attempt_count"`
}

type diagnosticsJSON struct {
	Version    string                         `json:"version"`
	Engine     string                         `json:"engine"`
	WatchDirs  []string                       `json:"watch_dirs"`
	OutDir     string                         `json:"out_dir"`
	InPlace    bool                           `json:"in_place"`
	MaxRetries int                            `json:"max_retries"`
	Auth       diagnosticsAuthJSON            `json:"auth"`
	Tools      map[string]diagnosticsToolJSON `json:"tools"`
	Jobs       []diagnosticsJobJSON           `json:"jobs"`
}

type diagnosticsProbe struct {
	name   string
	result diagnosticsToolJSON
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	slog.Info("diagnostics export started", "operation", "diagnostics_export")

	settings, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		slog.Error("diagnostics settings load failed",
			"operation", "diagnostics_export",
			"duration_ms", time.Since(started).Milliseconds(),
			"error", err.Error(),
		)
		writeError(w, http.StatusInternalServerError, "load diagnostics settings")
		return
	}
	if s.deps.RuntimeSettings != nil {
		runtime := s.deps.RuntimeSettings()
		runtime.Auth = settings.Auth
		settings = runtime
	}
	if settings.Engine == "" {
		settings.Engine = s.deps.Engine
	}
	if len(settings.WatchDirs) == 0 {
		settings.WatchDirs = append([]string(nil), s.deps.WatchDirs...)
	}

	jobs, err := s.deps.Jobs.List(r.Context(), diagnosticsJobLimit)
	if err != nil {
		slog.Error("diagnostics jobs load failed",
			"operation", "diagnostics_export",
			"duration_ms", time.Since(started).Milliseconds(),
			"error", err.Error(),
		)
		writeError(w, http.StatusInternalServerError, "load diagnostics jobs")
		return
	}

	tools := s.probeDiagnosticsTools(r.Context(), settings.Auth)
	out := diagnosticsJSON{
		Version:    s.deps.Version,
		Engine:     settings.Engine,
		WatchDirs:  append([]string(nil), settings.WatchDirs...),
		OutDir:     settings.OutDir,
		InPlace:    settings.InPlace,
		MaxRetries: settings.MaxAttempts(),
		Auth: diagnosticsAuthJSON{
			PasswordConfigured: settings.Auth.PasswordHash != "",
			APIKeySet:          settings.Auth.APIKey != "",
			OIDCEnabled:        settings.Auth.OIDCEnabled,
		},
		Tools: tools,
		Jobs:  make([]diagnosticsJobJSON, 0, len(jobs)),
	}
	for _, job := range jobs {
		out.Jobs = append(out.Jobs, diagnosticsJobToJSON(job, settings.Auth))
	}

	writeJSON(w, http.StatusOK, out)
	slog.Info("diagnostics export completed",
		"operation", "diagnostics_export",
		"jobs", len(out.Jobs),
		"duration_ms", time.Since(started).Milliseconds(),
	)
}

func (s *Server) probeDiagnosticsTools(ctx context.Context, auth domain.AuthSettings) map[string]diagnosticsToolJSON {
	checks := []struct {
		name  string
		check func(context.Context) error
	}{
		{name: "shntool", check: s.deps.CheckShntool},
		{name: "metaflac", check: s.deps.CheckMetaflac},
	}
	results := make(chan diagnosticsProbe, len(checks))
	for _, item := range checks {
		go func() {
			started := time.Now()
			result := diagnosticsToolJSON{Available: true}
			if item.check == nil {
				result.Available = false
				result.Error = "probe not configured"
			} else if err := item.check(ctx); err != nil {
				result.Available = false
				result.Error = redactKnownSecrets(err.Error(), auth)
			}
			slog.Debug("diagnostics tool probe completed",
				"operation", "diagnostics_export",
				"tool", item.name,
				"available", result.Available,
				"duration_ms", time.Since(started).Milliseconds(),
			)
			results <- diagnosticsProbe{name: item.name, result: result}
		}()
	}

	tools := make(map[string]diagnosticsToolJSON, len(checks))
	for range checks {
		probe := <-results
		tools[probe.name] = probe.result
	}
	return tools
}

func diagnosticsJobToJSON(job domain.Job, auth domain.AuthSettings) diagnosticsJobJSON {
	logText := redactJobText(job.Log, job, auth)
	return diagnosticsJobJSON{
		ID:           job.ID,
		Status:       job.Status,
		AlbumLabel:   albumLabel(job),
		CueFile:      filepath.Base(job.CuePath),
		ImageFile:    filepath.Base(job.ImagePath),
		Log:          truncateUTF8(logText, diagnosticsLogLimit),
		Error:        redactJobText(job.Error, job, auth),
		AttemptCount: job.AttemptCount,
	}
}

func albumLabel(job domain.Job) string {
	path := job.CuePath
	if path == "" {
		path = job.ImagePath
	}
	if path == "" {
		return ""
	}
	return filepath.Base(filepath.Dir(path))
}

func redactJobText(text string, job domain.Job, auth domain.AuthSettings) string {
	text = redactKnownSecrets(text, auth)
	for _, path := range []string{job.CuePath, job.ImagePath, job.OutDir} {
		if path != "" {
			text = strings.ReplaceAll(text, path, filepath.Base(path))
		}
	}
	return text
}

func redactKnownSecrets(text string, auth domain.AuthSettings) string {
	for _, secret := range []string{auth.PasswordHash, auth.APIKey, auth.OIDCClientSecret} {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return text
}

func truncateUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	text = text[:limit]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}
