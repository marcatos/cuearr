package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/auth"
	httpapi "github.com/marcatos/cuearr/internal/adapters/http"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type memJobStore struct {
	jobs map[string]domain.Job
}

func (m *memJobStore) Create(_ context.Context, job domain.Job) (domain.Job, error) {
	if m.jobs == nil {
		m.jobs = make(map[string]domain.Job)
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *memJobStore) Get(_ context.Context, id string) (domain.Job, error) {
	j, ok := m.jobs[id]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return j, nil
}

func (m *memJobStore) List(_ context.Context, _ int) ([]domain.Job, error) {
	out := make([]domain.Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out, nil
}

func (m *memJobStore) Update(_ context.Context, job domain.Job) error {
	if _, ok := m.jobs[job.ID]; !ok {
		return domain.ErrNotFound
	}
	m.jobs[job.ID] = job
	return nil
}

func (m *memJobStore) FindByFingerprint(_ context.Context, fp string) (domain.Job, error) {
	for _, j := range m.jobs {
		if j.Fingerprint == fp {
			return j, nil
		}
	}
	return domain.Job{}, domain.ErrNotFound
}

func (m *memJobStore) ClaimNextQueued(_ context.Context, startedAt time.Time) (domain.Job, error) {
	var (
		found domain.Job
		ok    bool
	)
	for _, j := range m.jobs {
		if j.Status != domain.JobQueued {
			continue
		}
		if !ok || j.CreatedAt.Before(found.CreatedAt) {
			found = j
			ok = true
		}
	}
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	found.Status = domain.JobRunning
	found.StartedAt = startedAt
	m.jobs[found.ID] = found
	return found, nil
}

var _ ports.JobStore = (*memJobStore)(nil)

type memSettingsStore struct {
	s domain.Settings
}

func (m *memSettingsStore) Get(context.Context) (domain.Settings, error) {
	return m.s, nil
}

func (m *memSettingsStore) Put(_ context.Context, s domain.Settings) error {
	m.s = s
	return nil
}

var _ ports.SettingsStore = (*memSettingsStore)(nil)

const testAPIKey = "test-api-key"

func newTestServer(t *testing.T, jobs ports.JobStore, createJob func(context.Context, string) (domain.Job, bool, error)) *httptest.Server {
	t.Helper()
	settings := &memSettingsStore{
		s: domain.Settings{
			Auth: domain.AuthSettings{
				PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
				APIKey:       testAPIKey,
			},
		},
	}
	srv := httpapi.New(httpapi.Deps{
		Jobs:          jobs,
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		Engine:        "shntool",
		WatchDirs:     []string{"/watch"},
		CreateJob:     createJob,
		ScanWatch:     func(context.Context) error { return nil },
	})
	return httptest.NewServer(srv.Handler())
}

func apiGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", testAPIKey)
	return http.DefaultClient.Do(req)
}

func apiPost(url, contentType string, body *bytes.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Api-Key", testAPIKey)
	return http.DefaultClient.Do(req)
}

func TestHealth_OK(t *testing.T) {
	ts := newTestServer(t, &memJobStore{}, nil)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true || body["sqlite_ok"] != true {
		t.Fatalf("body=%v", body)
	}
}

func TestCreateJob(t *testing.T) {
	now := time.Now().UTC()
	want := domain.Job{
		ID:        "job-1",
		Status:    domain.JobQueued,
		CuePath:   "/album/album.cue",
		ImagePath: "/album/album.flac",
		Engine:    "shntool",
		CreatedAt: now,
	}
	jobs := &memJobStore{}
	ts := newTestServer(t, jobs, func(_ context.Context, path string) (domain.Job, bool, error) {
		if path != "/album/dir" {
			t.Fatalf("path=%q", path)
		}
		jobs.jobs = map[string]domain.Job{want.ID: want}
		return want, true, nil
	})
	defer ts.Close()

	payload := []byte(`{"path":"/album/dir"}`)
	res, err := apiPost(ts.URL+"/api/v1/jobs", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var body struct {
		Job     json.RawMessage `json:"job"`
		Created bool            `json:"created"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Created {
		t.Fatal("expected created=true")
	}
}

func TestListJobs(t *testing.T) {
	attemptedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	jobs := &memJobStore{
		jobs: map[string]domain.Job{
			"a": {
				ID: "a", Status: domain.JobQueued, CreatedAt: time.Now().UTC(),
				AttemptCount: 1,
				AttemptLog:   []domain.JobAttempt{{Number: 1, At: attemptedAt, Error: "split failed"}},
			},
			"b": {ID: "b", Status: domain.JobCompleted, CreatedAt: time.Now().UTC()},
		},
	}
	ts := newTestServer(t, jobs, nil)
	defer ts.Close()

	res, err := apiGet(ts.URL + "/api/v1/jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var body struct {
		Jobs []struct {
			ID           string `json:"id"`
			AttemptCount int    `json:"attempt_count"`
			Attempts     []struct {
				Number int    `json:"n"`
				At     string `json:"at"`
				Error  string `json:"error"`
			} `json:"attempts"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Jobs) != 2 {
		t.Fatalf("jobs=%d", len(body.Jobs))
	}
	for _, job := range body.Jobs {
		if job.ID == "a" {
			if job.AttemptCount != 1 || len(job.Attempts) != 1 ||
				job.Attempts[0].Number != 1 || job.Attempts[0].At != attemptedAt.Format(time.RFC3339Nano) ||
				job.Attempts[0].Error != "split failed" {
				t.Fatalf("attempt fields=%+v", job)
			}
			return
		}
	}
	t.Fatal("job a missing")
}

func TestScanJobs_ReturnsAcceptedBeforeScanCompletes(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	settings := &memSettingsStore{
		s: domain.Settings{Auth: domain.AuthSettings{APIKey: testAPIKey}},
	}
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		WatchDirs:     []string{"/watch"},
		ScanWatch: func(context.Context) error {
			close(started)
			<-release
			return nil
		},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	defer close(release)

	done := make(chan *http.Response, 1)
	go func() {
		res, err := apiPost(ts.URL+"/api/v1/jobs/scan", "application/json", bytes.NewReader(nil))
		if err != nil {
			t.Error(err)
			done <- nil
			return
		}
		done <- res
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not start")
	}

	select {
	case res := <-done:
		if res == nil {
			t.Fatal("request failed")
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusAccepted {
			t.Fatalf("status=%d", res.StatusCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler blocked on scan")
	}
}

func TestPutSettings_AppliesRuntimeConfig(t *testing.T) {
	settings := &memSettingsStore{
		s: domain.Settings{Auth: domain.AuthSettings{APIKey: testAPIKey}},
	}
	var applied domain.Settings
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		ApplySettings: func(_ context.Context, next domain.Settings) error {
			applied = next
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(
		`{"watch_dirs":["/new/watch"],"out_dir":"/new/out","in_place":true,"engine":"shntool","max_retries":4}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if applied.Engine != "shntool" || applied.OutDir != "/new/out" || !applied.InPlace ||
		len(applied.WatchDirs) != 1 || applied.WatchDirs[0] != "/new/watch" || applied.MaxRetries != 4 {
		t.Fatalf("applied=%+v", applied)
	}
}

func TestPutSettings_RejectsNativeEngine(t *testing.T) {
	settings := &memSettingsStore{
		s: domain.Settings{
			Engine: "shntool",
			Auth:   domain.AuthSettings{APIKey: testAPIKey},
		},
	}
	applied := false
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		ApplySettings: func(context.Context, domain.Settings) error {
			applied = true
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(
		`{"engine":"native","max_retries":3}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if applied {
		t.Fatal("unsupported settings were applied")
	}
	if settings.s.Engine != "shntool" {
		t.Fatalf("stored engine=%q", settings.s.Engine)
	}
}

func TestLogout_ClearsSessionCookie(t *testing.T) {
	ts := newTestServer(t, &memJobStore{}, nil)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var cleared *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == auth.SessionCookieName {
			cleared = c
			break
		}
	}
	if cleared == nil {
		t.Fatalf("Set-Cookie missing %q; headers=%v", auth.SessionCookieName, res.Header["Set-Cookie"])
	}
	if cleared.MaxAge != -1 {
		t.Fatalf("MaxAge=%d want -1", cleared.MaxAge)
	}
	if cleared.HttpOnly != auth.ClearSessionCookie().HttpOnly {
		t.Fatal("expected HttpOnly clear cookie")
	}
}

func TestStaticRoot_ReturnsHTML(t *testing.T) {
	settings := &memSettingsStore{
		s: domain.Settings{
			Auth: domain.AuthSettings{
				PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
				APIKey:       testAPIKey,
			},
		},
	}
	srv := httpapi.New(httpapi.Deps{
		Jobs:          &memJobStore{},
		Settings:      settings,
		SessionSecret: []byte("test-session-secret"),
		Engine:        "shntool",
		WatchDirs:     []string{"/watch"},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type=%q", ct)
	}
	body := rec.Body.String()
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "<html") && !strings.Contains(lower, "<!doctype html") {
		end := len(body)
		if end > 200 {
			end = 200
		}
		t.Fatalf("body=%q", body[:end])
	}
}
