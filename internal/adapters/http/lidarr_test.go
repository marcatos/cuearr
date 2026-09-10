package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestExtractLidarrScanPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload string
		want    string
		wantErr bool
	}{
		{
			name:    "simple path",
			payload: `{"path":"/music/Artist/Album/track.flac"}`,
			want:    "/music/Artist/Album/track.flac",
		},
		{
			name:    "top-level DownloadPath",
			payload: `{"DownloadPath":"/downloads/album"}`,
			want:    "/downloads/album",
		},
		{
			name:    "top-level DownloadFolder",
			payload: `{"DownloadFolder":"/downloads/album"}`,
			want:    "/downloads/album",
		},
		{
			name:    "environment DownloadPath",
			payload: `{"eventType":"Download","environment":{"DownloadPath":"/staging/Artist - Album"}}`,
			want:    "/staging/Artist - Album",
		},
		{
			name:    "environment DownloadFolder",
			payload: `{"environment":{"DownloadFolder":"/import/ready"}}`,
			want:    "/import/ready",
		},
		{
			name:    "env DownloadPath",
			payload: `{"env":{"DownloadPath":"/via-env"}}`,
			want:    "/via-env",
		},
		{
			name:    "path wins over nested when both present",
			payload: `{"path":"/explicit","environment":{"DownloadPath":"/ignored"}}`,
			want:    "/explicit",
		},
		{
			name:    "lidarr webhook noise",
			payload: `{"eventType":"Download","instanceName":"Lidarr","applicationUrl":"http://lidarr:8686","environment":{"DownloadPath":"/album"}}`,
			want:    "/album",
		},
		{
			name:    "trackFiles first path",
			payload: `{"eventType":"TrackRetag","trackFiles":[{"path":"/music/Artist/Album/01.flac","quality":{"quality":{"name":"FLAC"}}},{"path":"/music/Artist/Album/02.flac"}]}`,
			want:    "/music/Artist/Album/01.flac",
		},
		{
			name:    "environment lidarr_trackfile_path",
			payload: `{"environment":{"lidarr_trackfile_path":"/import/Artist/01 Track.flac"}}`,
			want:    "/import/Artist/01 Track.flac",
		},
		{
			name:    "environment Lidarr_AddedTrackPaths pipe list",
			payload: `{"environment":{"Lidarr_AddedTrackPaths":"/a/one.flac|/a/two.flac"}}`,
			want:    "/a/one.flac",
		},
		{
			name:    "top-level lidarr_release_path",
			payload: `{"lidarr_release_path":"/library/Artist/Album"}`,
			want:    "/library/Artist/Album",
		},
		{
			name:    "empty path string",
			payload: `{"path":""}`,
			wantErr: true,
		},
		{
			name:    "no known keys",
			payload: `{"eventType":"Test"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			payload: `{not json`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractLidarrScanPath([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("path=%q want %q", got, tt.want)
			}
		})
	}
}

func TestLidarrDirForScan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "album.flac")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "existing directory unchanged",
			path: dir,
			want: dir,
		},
		{
			name: "existing file becomes parent dir",
			path: file,
			want: dir,
		},
		{
			name: "missing file path with extension becomes parent",
			path: filepath.Join(dir, "missing.cue"),
			want: dir,
		},
		{
			name: "missing path without extension kept as-is",
			path: filepath.Join(dir, "missingdir"),
			want: filepath.Join(dir, "missingdir"),
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := lidarrDirForScan(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Clean(tt.want)
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestHandleLidarrHook(t *testing.T) {
	wantJob := domain.Job{
		ID:        "job-lidarr-1",
		Status:    domain.JobQueued,
		CuePath:   "/album/album.cue",
		ImagePath: "/album/album.flac",
		Engine:    "shntool",
		CreatedAt: time.Now().UTC(),
	}
	var gotPath string
	srv := New(Deps{
		Jobs:          &memJobStore{},
		Settings:      &memSettingsStore{s: domain.Settings{Auth: domain.AuthSettings{APIKey: testAPIKey}}},
		SessionSecret: []byte("secret"),
		Engine:        "shntool",
		CreateJob: func(_ context.Context, path string) (domain.Job, bool, error) {
			gotPath = path
			return wantJob, true, nil
		},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := []byte(`{"environment":{"DownloadPath":"/album/dir"}}`)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/hooks/lidarr", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", res.StatusCode)
	}
	if gotPath != filepath.Clean("/album/dir") {
		t.Fatalf("scan path=%q", gotPath)
	}
	var body struct {
		JobID   string `json:"job_id"`
		Created bool   `json:"created"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.JobID != wantJob.ID || !body.Created {
		t.Fatalf("body=%+v", body)
	}
}

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
	return nil, nil
}

func (m *memJobStore) Update(_ context.Context, job domain.Job) error { return nil }

func (m *memJobStore) FindByFingerprint(_ context.Context, _ string) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

type memSettingsStore struct {
	s domain.Settings
}

func (m *memSettingsStore) Get(context.Context) (domain.Settings, error) { return m.s, nil }
func (m *memSettingsStore) Put(_ context.Context, s domain.Settings) error {
	m.s = s
	return nil
}

const testAPIKey = "test-api-key"
