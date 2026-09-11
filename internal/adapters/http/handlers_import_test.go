package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpapi "github.com/marcatos/cuearr/internal/adapters/http"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestGetJob_IncludesImportFields(t *testing.T) {
	requestedAt := time.Date(2026, 9, 11, 7, 0, 0, 0, time.UTC)
	finishedAt := requestedAt.Add(time.Second)
	jobs := &memJobStore{jobs: map[string]domain.Job{
		"job-1": {
			ID:                "job-1",
			Status:            domain.JobCompleted,
			ImportStatus:      domain.ImportFailed,
			ImportError:       "Lidarr unavailable",
			ImportRequestedAt: requestedAt,
			ImportFinishedAt:  finishedAt,
			CreatedAt:         requestedAt.Add(-time.Minute),
		},
	}}
	ts := newTestServer(t, jobs, nil)
	defer ts.Close()

	res, err := apiGet(ts.URL + "/api/v1/jobs/job-1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var body struct {
		ImportStatus      string `json:"import_status"`
		ImportError       string `json:"import_error"`
		ImportRequestedAt string `json:"import_requested_at"`
		ImportFinishedAt  string `json:"import_finished_at"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ImportStatus != domain.ImportFailed || body.ImportError != "Lidarr unavailable" ||
		body.ImportRequestedAt != requestedAt.Format(time.RFC3339Nano) ||
		body.ImportFinishedAt != finishedAt.Format(time.RFC3339Nano) {
		t.Fatalf("import fields=%+v", body)
	}
}

func TestImportJob_RequestsImport(t *testing.T) {
	settings := &memSettingsStore{s: domain.Settings{
		Auth: domain.AuthSettings{APIKey: testAPIKey},
	}}
	want := domain.Job{ID: "job-1", Status: domain.JobCompleted, ImportStatus: domain.ImportImported}
	var requestedID string
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{jobs: map[string]domain.Job{"job-1": want}},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		RequestImport: func(_ context.Context, id string) (domain.Job, error) {
			requestedID = id
			return want, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/job-1/import", nil)
	req.Header.Set("X-Api-Key", testAPIKey)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if requestedID != "job-1" {
		t.Fatalf("requested id=%q", requestedID)
	}
	var body struct {
		ID           string `json:"id"`
		ImportStatus string `json:"import_status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ID != want.ID || body.ImportStatus != domain.ImportImported {
		t.Fatalf("body=%+v", body)
	}
}

func TestSettings_LidarrFieldsAndAPIKeyMasking(t *testing.T) {
	settings := &memSettingsStore{s: domain.Settings{
		Engine:                "shntool",
		LidarrURL:             "http://lidarr.internal:8686/base",
		LidarrAPIKey:          "lidarr-secret",
		LidarrImportEnabled:   true,
		LidarrPathMap:         []domain.PathMapRule{{From: "/downloads", To: "/data"}},
		LidarrPollIntervalSec: 300,
		Auth:                  domain.AuthSettings{APIKey: testAPIKey},
	}}
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
	})

	get := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	get.Header.Set("X-Api-Key", testAPIKey)
	getRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRec, get)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var got struct {
		LidarrURL             string               `json:"lidarr_url"`
		LidarrAPIKey          string               `json:"lidarr_api_key"`
		LidarrAPIKeySet       bool                 `json:"lidarr_api_key_set"`
		LidarrImportEnabled   bool                 `json:"lidarr_import_enabled"`
		LidarrPathMap         []domain.PathMapRule `json:"lidarr_path_map"`
		LidarrPollIntervalSec int                  `json:"lidarr_poll_interval_sec"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.LidarrURL != settings.s.LidarrURL || got.LidarrAPIKey != "" || !got.LidarrAPIKeySet ||
		!got.LidarrImportEnabled || len(got.LidarrPathMap) != 1 || got.LidarrPollIntervalSec != 300 {
		t.Fatalf("GET settings=%+v", got)
	}
	if strings.Contains(getRec.Body.String(), settings.s.LidarrAPIKey) {
		t.Fatal("GET settings leaked Lidarr API key")
	}

	put := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(
		`{"engine":"shntool","max_retries":3,"lidarr_url":"http://new-lidarr:8686",`+
			`"lidarr_api_key":"","lidarr_import_enabled":true,`+
			`"lidarr_path_map":[{"from":"C:\\downloads","to":"/downloads"}],`+
			`"lidarr_poll_interval_sec":600}`,
	))
	put.Header.Set("Content-Type", "application/json")
	put.Header.Set("X-Api-Key", testAPIKey)
	putRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(putRec, put)

	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", putRec.Code, putRec.Body.String())
	}
	if settings.s.LidarrAPIKey != "lidarr-secret" || settings.s.LidarrURL != "http://new-lidarr:8686" ||
		settings.s.LidarrPollIntervalSec != 600 || len(settings.s.LidarrPathMap) != 1 {
		t.Fatalf("stored settings=%+v", settings.s)
	}

	updateKey := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(
		`{"engine":"shntool","max_retries":3,"lidarr_api_key":"replacement-secret"}`,
	))
	updateKey.Header.Set("Content-Type", "application/json")
	updateKey.Header.Set("X-Api-Key", testAPIKey)
	updateKeyRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(updateKeyRec, updateKey)
	if updateKeyRec.Code != http.StatusOK {
		t.Fatalf("key PUT status=%d body=%s", updateKeyRec.Code, updateKeyRec.Body.String())
	}
	if settings.s.LidarrAPIKey != "replacement-secret" {
		t.Fatalf("stored Lidarr API key=%q", settings.s.LidarrAPIKey)
	}
	if strings.Contains(updateKeyRec.Body.String(), "replacement-secret") {
		t.Fatal("PUT settings response leaked replacement Lidarr API key")
	}
}
