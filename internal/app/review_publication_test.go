package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type completionVisibilityStore struct {
	*fakeJobStore
	t *testing.T
}

func (s *completionVisibilityStore) Update(ctx context.Context, job domain.Job) error {
	if err := s.fakeJobStore.Update(ctx, job); err != nil {
		return err
	}
	if job.Status == domain.JobCompleted {
		if _, err := os.Stat(filepath.Join(job.OutDir, "01.flac")); err != nil {
			s.t.Errorf("completed job is visible in the store before its output exists: %v", err)
		}
	}
	return nil
}

func TestReview_CompletedJobHasPublishedOutput(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	job := domain.Job{
		ID: "completion-visibility", Fingerprint: "completion-visibility",
		CuePath: writeCue(t, oneTrackCue), ImagePath: filepath.Join(root, "album.flac"),
		Status: domain.JobQueued,
	}
	if err := os.WriteFile(job.ImagePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &completionVisibilityStore{fakeJobStore: newImportStore(t, job), t: t}
	runner := app.JobRunner{
		Store:    store,
		Splitter: &fakeSplitter{result: ports.SplitResult{OutputFiles: []string{"01.flac"}}},
		Inspector: &fakeFLACInspector{info: map[string]ports.FLACInfo{
			job.ImagePath: {Duration: 3 * time.Second},
			"01.flac":     {Duration: 3 * time.Second},
		}},
		Tagger: &fakeFLACTagger{}, Preflight: &fakePreflight{},
	}
	if _, err := runner.RunJob(ctx, job, filepath.Join(root, "out"), false); err != nil {
		t.Fatal(err)
	}
}
