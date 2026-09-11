package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	httpapi "github.com/marcatos/cuearr/internal/adapters/http"
	"github.com/marcatos/cuearr/internal/domain"
)

type diagnosticsJobStore struct {
	*memJobStore
	listLimit int
}

func (s *diagnosticsJobStore) List(ctx context.Context, limit int) ([]domain.Job, error) {
	s.listLimit = limit
	return s.memJobStore.List(ctx, limit)
}

func TestDiagnostics_RequiresAuthentication(t *testing.T) {
	settings := &memSettingsStore{s: domain.Settings{
		Auth: domain.AuthSettings{APIKey: testAPIKey},
	}}
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDiagnostics_ReturnsRedactedSnapshot(t *testing.T) {
	const (
		passwordHash = "$2a$10$diagnostics-password-hash"
		apiKey       = "diagnostics-api-key-secret"
		oidcSecret   = "diagnostics-oidc-secret"
	)
	cuePath := filepath.Join("private", "Artist", "Album", "disc.cue")
	imagePath := filepath.Join("private", "Artist", "Album", "disc.flac")
	longLog := strings.Repeat("x", 3*1024)
	jobs := &diagnosticsJobStore{memJobStore: &memJobStore{
		jobs: map[string]domain.Job{
			"job-1": {
				ID:           "job-1",
				Fingerprint:  "private-fingerprint",
				CuePath:      cuePath,
				ImagePath:    imagePath,
				OutDir:       filepath.Join("private", "output"),
				Status:       domain.JobFailed,
				Log:          longLog,
				Error:        "split failed lidarr-api-key-secret",
				AttemptCount: 2,
			},
		},
	}}
	settings := &memSettingsStore{s: domain.Settings{
		WatchDirs:           []string{filepath.Join("private", "watch")},
		OutDir:              filepath.Join("private", "out"),
		InPlace:             true,
		Engine:              "shntool",
		MaxRetries:          4,
		LidarrURL:           "https://lidarr-user:lidarr-password@lidarr.internal:8686/private/path?token=secret",
		LidarrAPIKey:        "lidarr-api-key-secret",
		LidarrImportEnabled: true,
		Auth: domain.AuthSettings{
			PasswordHash:     passwordHash,
			APIKey:           apiKey,
			OIDCEnabled:      true,
			OIDCClientSecret: oidcSecret,
		},
	}}
	srv := httpapi.New(httpapi.Deps{
		Jobs:          jobs,
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		Version:       "v0.3.0-test",
		CheckShntool:  func(context.Context) error { return nil },
		CheckMetaflac: func(context.Context) error { return errors.New("metaflac unavailable") },
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics", nil)
	req.Header.Set("X-Api-Key", apiKey)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if jobs.listLimit != 20 {
		t.Fatalf("job list limit=%d want 20", jobs.listLimit)
	}
	raw := rec.Body.String()
	for _, secret := range []string{
		passwordHash, apiKey, oidcSecret, settings.s.LidarrAPIKey,
		"lidarr-user", "lidarr-password", "/private/path", "token=secret",
		cuePath, imagePath, "private-fingerprint",
	} {
		if strings.Contains(raw, secret) {
			t.Fatalf("diagnostics leaked %q: %s", secret, raw)
		}
	}

	var body struct {
		Version             string   `json:"version"`
		Engine              string   `json:"engine"`
		WatchDirs           []string `json:"watch_dirs"`
		OutDir              string   `json:"out_dir"`
		InPlace             bool     `json:"in_place"`
		MaxRetries          int      `json:"max_retries"`
		LidarrImportEnabled bool     `json:"lidarr_import_enabled"`
		LidarrHost          string   `json:"lidarr_host"`
		Auth                struct {
			PasswordConfigured bool `json:"password_configured"`
			APIKeySet          bool `json:"api_key_set"`
			OIDCEnabled        bool `json:"oidc_enabled"`
		} `json:"auth"`
		Tools map[string]struct {
			Available bool   `json:"available"`
			Error     string `json:"error"`
		} `json:"tools"`
		Jobs []struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			AlbumLabel   string `json:"album_label"`
			CueFile      string `json:"cue_file"`
			ImageFile    string `json:"image_file"`
			Log          string `json:"log"`
			Error        string `json:"error"`
			AttemptCount int    `json:"attempt_count"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "v0.3.0-test" || body.Engine != "shntool" ||
		len(body.WatchDirs) != 1 || body.OutDir != settings.s.OutDir ||
		!body.InPlace || body.MaxRetries != 4 || !body.LidarrImportEnabled ||
		body.LidarrHost != "lidarr.internal:8686" {
		t.Fatalf("settings snapshot=%+v", body)
	}
	if !body.Auth.PasswordConfigured || !body.Auth.APIKeySet || !body.Auth.OIDCEnabled {
		t.Fatalf("auth flags=%+v", body.Auth)
	}
	if !body.Tools["shntool"].Available || body.Tools["shntool"].Error != "" {
		t.Fatalf("shntool probe=%+v", body.Tools["shntool"])
	}
	if body.Tools["metaflac"].Available || body.Tools["metaflac"].Error != "metaflac unavailable" {
		t.Fatalf("metaflac probe=%+v", body.Tools["metaflac"])
	}
	if len(body.Jobs) != 1 {
		t.Fatalf("jobs=%d want 1", len(body.Jobs))
	}
	job := body.Jobs[0]
	if job.ID != "job-1" || job.Status != domain.JobFailed || job.AlbumLabel != "Album" ||
		job.CueFile != "disc.cue" || job.ImageFile != "disc.flac" ||
		job.Error != "split failed [redacted]" || job.AttemptCount != 2 {
		t.Fatalf("redacted job=%+v", job)
	}
	if len(job.Log) != 2*1024 {
		t.Fatalf("log bytes=%d want %d", len(job.Log), 2*1024)
	}
}
