package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func newTestServer(t *testing.T, jobs ports.JobStore, createJob func(context.Context, string) (domain.Job, bool, error)) *httptest.Server {
	t.Helper()
	srv := httpapi.New(httpapi.Deps{
		Jobs:      jobs,
		Settings:  &memSettingsStore{},
		Engine:    "shntool",
		WatchDirs: []string{"/watch"},
		CreateJob: createJob,
		ScanWatch: func(context.Context) error { return nil },
	})
	return httptest.NewServer(srv.Handler())
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
	res, err := http.Post(ts.URL+"/api/v1/jobs", "application/json", bytes.NewReader(payload))
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
	jobs := &memJobStore{
		jobs: map[string]domain.Job{
			"a": {ID: "a", Status: domain.JobQueued, CreatedAt: time.Now().UTC()},
			"b": {ID: "b", Status: domain.JobCompleted, CreatedAt: time.Now().UTC()},
		},
	}
	ts := newTestServer(t, jobs, nil)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var body struct {
		Jobs []struct {
			ID string `json:"id"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Jobs) != 2 {
		t.Fatalf("jobs=%d", len(body.Jobs))
	}
}
