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

	"github.com/marcatos/cuearr/internal/adapters/splitter/shntool"
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
		t.Skipf("fixture cue missing (%v); run scripts/generate_fixture.sh", err)
	}
	if _, err := os.Stat(flac); err != nil {
		t.Skipf("fixture flac missing (%v); run scripts/generate_fixture.sh or scripts/generate_fixture.ps1", err)
	}
	return dir
}

func TestE2E_SplitFixtureWithShntool(t *testing.T) {
	if _, err := exec.LookPath("shntool"); err != nil {
		t.Skip("shntool not in PATH")
	}
	albumDir := e2eAlbumDir(t)
	outDir := t.TempDir()

	ctx := context.Background()
	splitter := shntool.New(e2eExecRunner{}, "shntool")
	if err := splitter.Available(ctx); err != nil {
		t.Skipf("shntool not usable: %v", err)
	}

	plan := domain.SplitPlan{
		CuePath:   filepath.Join(albumDir, "album.cue"),
		ImagePath: filepath.Join(albumDir, "album.flac"),
		WorkDir:   albumDir,
	}
	result, err := splitter.Split(ctx, plan, outDir)
	if err != nil {
		t.Fatalf("split: %v (log=%q)", err, result.Log)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	var flacs int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".flac") {
			flacs++
		}
	}
	if flacs < 2 {
		t.Fatalf("expected at least 2 track flacs in %s, got %d (log=%q)", outDir, flacs, result.Log)
	}
}
