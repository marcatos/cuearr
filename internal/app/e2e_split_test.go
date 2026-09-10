package app_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/audio/metaflac"
	"github.com/marcatos/cuearr/internal/adapters/splitter/shntool"
	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

type e2eExecRunner struct{}

func (e2eExecRunner) Run(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	exitCode = 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return outBuf.String(), errBuf.String(), -1, runErr
		}
	}
	return outBuf.String(), errBuf.String(), exitCode, nil
}

func e2eAlbumDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "e2e_album")
	cue := filepath.Join(dir, "album.cue")
	flac := filepath.Join(dir, "album.flac")
	if _, err := os.Stat(cue); err != nil {
		e2eUnavailable(t, "fixture cue missing (%v); run scripts/generate_fixture.sh", err)
	}
	if _, err := os.Stat(flac); err != nil {
		e2eUnavailable(t, "fixture flac missing (%v); run scripts/generate_fixture.sh or scripts/generate_fixture.ps1", err)
	}
	return dir
}

func e2eUnavailable(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("CUEARR_REQUIRE_SHNTOOL") == "1" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

func requireE2EExecutable(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		e2eUnavailable(t, "%s not in PATH: %v", name, err)
	}
}

func TestE2E_RunJobSplitsVerifiesAndTagsFixture(t *testing.T) {
	requireE2EExecutable(t, "shntool")
	requireE2EExecutable(t, "metaflac")
	albumDir := e2eAlbumDir(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	commandRunner := e2eExecRunner{}
	splitter := shntool.New(commandRunner, "shntool")
	if err := splitter.Available(ctx); err != nil {
		e2eUnavailable(t, "shntool not usable: %v", err)
	}
	audio := metaflac.New(commandRunner, "metaflac")
	store := &fakeJobStore{}
	job := domain.Job{
		ID:          "e2e-synthetic-album",
		Fingerprint: "e2e-synthetic-album",
		CuePath:     filepath.Join(albumDir, "album.cue"),
		ImagePath:   filepath.Join(albumDir, "album.flac"),
		Status:      domain.JobQueued,
		Engine:      splitter.Name(),
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatalf("create e2e job: %v", err)
	}
	runner := app.JobRunner{
		Store: store, Splitter: splitter, Inspector: audio, Tagger: audio,
	}
	finished, err := runner.RunJob(ctx, job, t.TempDir(), false)
	if err != nil {
		t.Fatalf("run job: %v (log=%q)", err, finished.Log)
	}
	if finished.Status != domain.JobCompleted {
		t.Fatalf("job status=%q, want %q", finished.Status, domain.JobCompleted)
	}

	entries, err := os.ReadDir(finished.OutDir)
	if err != nil {
		t.Fatalf("read job output: %v", err)
	}
	var tracks []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".flac") {
			tracks = append(tracks, filepath.Join(finished.OutDir, e.Name()))
		}
	}
	if len(tracks) != 2 {
		t.Fatalf("expected exactly 2 track flacs in %s, got %d (log=%q)", finished.OutDir, len(tracks), finished.Log)
	}

	expectedTags := [][4]string{
		{"TITLE=Track One", "ARTIST=Cuearr Test", "ALBUM=Fixture Album", "TRACKNUMBER=1"},
		{"TITLE=Track Two", "ARTIST=Cuearr Test", "ALBUM=Fixture Album", "TRACKNUMBER=2"},
	}
	for i, track := range tracks {
		info, inspectErr := audio.Inspect(ctx, track)
		if inspectErr != nil {
			t.Fatalf("inspect track %d: %v", i+1, inspectErr)
		}
		delta := info.Duration - 3*time.Second
		if delta < 0 {
			delta = -delta
		}
		if delta > 100*time.Millisecond {
			t.Errorf("track %d duration=%v, want 3s ±100ms", i+1, info.Duration)
		}
		for _, expected := range expectedTags[i] {
			field := strings.SplitN(expected, "=", 2)[0]
			stdout, stderr, exitCode, runErr := commandRunner.Run(ctx, "metaflac", "--show-tag="+field, track)
			if runErr != nil || exitCode != 0 {
				t.Fatalf("read %s tag from track %d: err=%v exit=%d stderr=%q", field, i+1, runErr, exitCode, stderr)
			}
			if got := strings.TrimSpace(stdout); got != expected {
				t.Errorf("track %d %s tag=%q, want %q", i+1, field, got, expected)
			}
		}
	}
}
