package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeSplitter struct {
	result  ports.SplitResult
	err     error
	outDirs []string
}

func (f *fakeSplitter) Name() string { return "fake" }

func (f *fakeSplitter) Available(context.Context) error { return nil }

func (f *fakeSplitter) Split(_ context.Context, _ domain.SplitPlan, outDir string) (ports.SplitResult, error) {
	f.outDirs = append(f.outDirs, outDir)
	if f.err != nil {
		return f.result, f.err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return ports.SplitResult{}, err
	}
	result := f.result
	result.OutputFiles = make([]string, 0, len(f.result.OutputFiles))
	for _, output := range f.result.OutputFiles {
		path := filepath.Join(outDir, filepath.Base(output))
		if err := os.WriteFile(path, []byte(filepath.Base(output)), 0o644); err != nil {
			return ports.SplitResult{}, err
		}
		result.OutputFiles = append(result.OutputFiles, path)
	}
	return result, nil
}

var _ ports.Splitter = (*fakeSplitter)(nil)

func TestRunJob_SuccessMarksCompletedAndLogsFiles(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	root := t.TempDir()
	imageDir := filepath.Join(root, "source")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(imageDir, "album.flac")
	if err := os.WriteFile(imagePath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := domain.Job{
		ID:          "job-run-1",
		Fingerprint: "fp-run",
		CuePath:     writeCue(t, twoTrackCue),
		ImagePath:   imagePath,
		Status:      domain.JobQueued,
		Engine:      "fake",
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}

	splitter := &fakeSplitter{
		result: ports.SplitResult{
			OutputFiles: []string{"/out/01.flac", "/out/02.flac"},
			Log:         "splitter stdout",
			Duration:    1500 * time.Millisecond,
		},
	}
	inspector := &fakeFLACInspector{info: map[string]ports.FLACInfo{
		job.ImagePath: {Duration: 6 * time.Second},
		"01.flac":     {Duration: 3 * time.Second},
		"02.flac":     {Duration: 3 * time.Second},
	}}
	tagger := &fakeFLACTagger{}
	runner := app.JobRunner{
		Store: store, Splitter: splitter, Inspector: inspector,
		Tagger: tagger, ReadFile: os.ReadFile,
	}

	baseOut := filepath.Join(root, "out")
	got, err := runner.RunJob(ctx, job, baseOut, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobCompleted {
		t.Fatalf("status=%q", got.Status)
	}
	if got.Error != "" {
		t.Fatalf("error=%q", got.Error)
	}
	if !strings.Contains(got.Log, "01.flac") || !strings.Contains(got.Log, "02.flac") {
		t.Fatalf("log=%q", got.Log)
	}
	if got.FinishedAt.IsZero() {
		t.Fatal("expected finished_at")
	}
	if got.FinishedAt.Before(tagger.calledAt) {
		t.Fatalf("finished_at=%v precedes final tag at %v", got.FinishedAt, tagger.calledAt)
	}
	staging := app.StagingDir(baseOut, job.ID)
	if len(splitter.outDirs) != 1 || splitter.outDirs[0] != staging {
		t.Fatalf("split out dirs=%v, want [%q]", splitter.outDirs, staging)
	}
	finalOut := filepath.Join(baseOut, job.Fingerprint)
	if got.OutDir != finalOut {
		t.Fatalf("out_dir=%q, want %q", got.OutDir, finalOut)
	}
	for _, name := range []string{"01.flac", "02.flac"} {
		if _, err := os.Stat(filepath.Join(finalOut, name)); err != nil {
			t.Fatalf("published %q: %v", name, err)
		}
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging remains: %v", err)
	}
	if content, err := os.ReadFile(job.ImagePath); err != nil || string(content) != "original" {
		t.Fatalf("source image changed: content=%q err=%v", content, err)
	}
}

func TestRunJob_InPlaceStagesBesideImageAndKeepsOriginals(t *testing.T) {
	ctx := context.Background()
	imageDir := t.TempDir()
	imagePath := filepath.Join(imageDir, "album.flac")
	if err := os.WriteFile(imagePath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := domain.Job{
		ID: "job-in-place", Fingerprint: "in-place", CuePath: writeCue(t, oneTrackCue),
		ImagePath: imagePath, Status: domain.JobQueued,
	}
	store := &fakeJobStore{}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	splitter := &fakeSplitter{result: ports.SplitResult{OutputFiles: []string{"01.flac"}}}
	runner := app.JobRunner{
		Store: store, Splitter: splitter,
		Inspector: &fakeFLACInspector{info: map[string]ports.FLACInfo{
			imagePath: {Duration: 3 * time.Second},
			"01.flac": {Duration: 3 * time.Second},
		}},
		Tagger: &fakeFLACTagger{}, ReadFile: os.ReadFile,
	}

	got, err := runner.RunJob(ctx, job, filepath.Join(t.TempDir(), "unused"), true)
	if err != nil {
		t.Fatal(err)
	}
	staging := app.StagingDir(imageDir, job.ID)
	if len(splitter.outDirs) != 1 || splitter.outDirs[0] != staging {
		t.Fatalf("split out dirs=%v, want [%q]", splitter.outDirs, staging)
	}
	if got.OutDir != imageDir {
		t.Fatalf("out_dir=%q, want %q", got.OutDir, imageDir)
	}
	for _, path := range []string{imagePath, filepath.Join(imageDir, "01.flac")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %q: %v", path, err)
		}
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging remains: %v", err)
	}
}

func TestRunJob_SplitErrorMarksFailed(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	job := domain.Job{
		ID:          "job-run-2",
		Fingerprint: "fp-run-fail",
		CuePath:     "/album/album.cue",
		ImagePath:   "/album/album.flac",
		Status:      domain.JobQueued,
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}

	splitter := &fakeSplitter{err: errors.New("split blew up")}
	runner := app.JobRunner{Store: store, Splitter: splitter}

	got, err := runner.RunJob(ctx, job, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if got.Status != domain.JobFailed {
		t.Fatalf("status=%q", got.Status)
	}
	if got.Error == "" {
		t.Fatal("expected error message on job")
	}
}

func TestRunJob_UsesDistinctPerAlbumOutputDirectories(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	output := "01.flac"
	splitter := &fakeSplitter{result: ports.SplitResult{OutputFiles: []string{output}}}
	jobs := []domain.Job{
		{ID: "job-a", Fingerprint: "album-a", CuePath: writeCue(t, oneTrackCue), ImagePath: "/music/a/album.flac", Status: domain.JobQueued},
		{ID: "job-b", Fingerprint: "album-b", CuePath: writeCue(t, oneTrackCue), ImagePath: "/music/b/album.flac", Status: domain.JobQueued},
	}
	inspector := &fakeFLACInspector{info: map[string]ports.FLACInfo{
		jobs[0].ImagePath: {Duration: 3 * time.Second},
		jobs[1].ImagePath: {Duration: 3 * time.Second},
		output:            {Duration: 3 * time.Second},
	}}
	runner := app.JobRunner{
		Store: store, Splitter: splitter, Inspector: inspector,
		Tagger: &fakeFLACTagger{}, ReadFile: os.ReadFile,
	}
	baseOut := t.TempDir()
	for _, job := range jobs {
		if _, err := store.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.RunJob(ctx, job, baseOut, false); err != nil {
			t.Fatal(err)
		}
	}
	if len(splitter.outDirs) != 2 {
		t.Fatalf("split calls=%d", len(splitter.outDirs))
	}
	if splitter.outDirs[0] == splitter.outDirs[1] {
		t.Fatalf("colliding output dirs: %q", splitter.outDirs[0])
	}
	if splitter.outDirs[0] != app.StagingDir(baseOut, jobs[0].ID) {
		t.Fatalf("first output dir=%q", splitter.outDirs[0])
	}
}

func TestRunJob_EmptySplitOutputMarksFailed(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	job := domain.Job{
		ID: "job-empty", Fingerprint: "fp-empty", CuePath: writeCue(t, oneTrackCue),
		ImagePath: "/album/image.flac", Status: domain.JobQueued,
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	runner := app.JobRunner{
		Store: store, Splitter: &fakeSplitter{},
		Inspector: &fakeFLACInspector{info: map[string]ports.FLACInfo{
			job.ImagePath: {Duration: 3 * time.Second},
		}},
		Tagger: &fakeFLACTagger{}, ReadFile: os.ReadFile,
	}

	baseOut := t.TempDir()
	got, err := runner.RunJob(ctx, job, baseOut, false)
	if err == nil {
		t.Fatal("expected empty output error")
	}
	if got.Status != domain.JobFailed {
		t.Fatalf("status=%q, want failed", got.Status)
	}
	if _, err := os.Stat(app.StagingDir(baseOut, job.ID)); !os.IsNotExist(err) {
		t.Fatalf("staging remains: %v", err)
	}
}

func TestRunJob_VerificationFailureMarksFailed(t *testing.T) {
	tests := []struct {
		name      string
		outputs   []string
		durations map[string]time.Duration
		tagErr    map[string]error
	}{
		{
			name:      "count mismatch",
			outputs:   []string{"01.flac"},
			durations: map[string]time.Duration{"01.flac": 3 * time.Second},
		},
		{
			name:    "duration mismatch",
			outputs: []string{"01.flac", "02.flac"},
			durations: map[string]time.Duration{
				"01.flac": 3200 * time.Millisecond,
				"02.flac": 3 * time.Second,
			},
		},
		{
			name:    "tag error",
			outputs: []string{"01.flac", "02.flac"},
			durations: map[string]time.Duration{
				"01.flac": 3 * time.Second,
				"02.flac": 3 * time.Second,
			},
			tagErr: map[string]error{"02.flac": errors.New("tag failed")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := &fakeJobStore{}
			job := domain.Job{
				ID: "job-" + strings.ReplaceAll(tc.name, " ", "-"), Fingerprint: tc.name,
				CuePath: writeCue(t, twoTrackCue), ImagePath: "/album/image.flac",
				Status: domain.JobQueued,
			}
			if _, err := store.Create(ctx, job); err != nil {
				t.Fatal(err)
			}
			info := map[string]ports.FLACInfo{
				job.ImagePath: {Duration: 6 * time.Second},
			}
			for path, duration := range tc.durations {
				info[path] = ports.FLACInfo{Duration: duration}
			}
			runner := app.JobRunner{
				Store:     store,
				Splitter:  &fakeSplitter{result: ports.SplitResult{OutputFiles: tc.outputs}},
				Inspector: &fakeFLACInspector{info: info},
				Tagger:    &fakeFLACTagger{err: tc.tagErr},
				ReadFile:  os.ReadFile,
			}

			baseOut := t.TempDir()
			got, err := runner.RunJob(ctx, job, baseOut, false)
			if err == nil {
				t.Fatal("expected verification error")
			}
			if got.Status != domain.JobFailed {
				t.Fatalf("status=%q, want failed", got.Status)
			}
			stored, getErr := store.Get(ctx, job.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if stored.Status != domain.JobFailed || stored.Error == "" {
				t.Fatalf("stored job status=%q error=%q", stored.Status, stored.Error)
			}
			if entries, readErr := os.ReadDir(got.OutDir); readErr == nil && len(entries) != 0 {
				t.Fatalf("final output contains files after failure: %v", entries)
			} else if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatalf("read final output: %v", readErr)
			}
			if _, statErr := os.Stat(app.StagingDir(baseOut, job.ID)); !os.IsNotExist(statErr) {
				t.Fatalf("staging remains: %v", statErr)
			}
		})
	}
}

const oneTrackCue = `PERFORMER "Artist"
TITLE "Album"
FILE "image.flac" WAVE
  TRACK 01 AUDIO
    TITLE "One"
    INDEX 01 00:00:00
`
