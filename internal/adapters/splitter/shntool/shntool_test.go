package shntool_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/splitter/shntool"
	"github.com/marcatos/cuearr/internal/domain"
)

type fakeRunner struct {
	exitCode int
	stdout   string
	stderr   string
	err      error
	lastName string
	lastArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	f.lastName = name
	f.lastArgs = append([]string(nil), args...)
	return f.stdout, f.stderr, f.exitCode, f.err
}

func TestShntoolSplit_BuildsExpectedArgs(t *testing.T) {
	fake := &fakeRunner{exitCode: 0, stdout: "ok"}
	s := shntool.New(fake, "shntool")
	plan := domain.SplitPlan{
		CuePath: "/in/album.cue", ImagePath: "/in/album.flac", WorkDir: "/in",
	}
	outDir := filepath.Join(t.TempDir(), "album")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "track01.flac"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := s.Split(context.Background(), plan, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outDir); err != nil {
		t.Fatalf("output directory not created: %v", err)
	}
	if fake.lastName != "shntool" {
		t.Fatalf("bin=%s", fake.lastName)
	}
	wantArgs := []string{
		"split",
		"-f", "/in/album.cue",
		"-o", "flac",
		"-d", outDir,
		"/in/album.flac",
	}
	if !reflect.DeepEqual(fake.lastArgs, wantArgs) {
		t.Fatalf("args=%v want=%v", fake.lastArgs, wantArgs)
	}
	_ = res
}

func TestShntoolSplit_NonzeroExitWrapsErrSplitFailed(t *testing.T) {
	const stderrMsg = "shntool: invalid cue sheet"
	fake := &fakeRunner{exitCode: 1, stderr: stderrMsg}
	s := shntool.New(fake, "shntool")
	plan := domain.SplitPlan{
		CuePath: "/in/album.cue", ImagePath: "/in/album.flac", WorkDir: "/in",
	}
	_, err := s.Split(context.Background(), plan, filepath.Join(t.TempDir(), "album"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrSplitFailed) {
		t.Fatalf("errors.Is: got %v", err)
	}
	if !strings.Contains(err.Error(), stderrMsg) {
		t.Fatalf("stderr not in error: %v", err)
	}
}

func TestShntoolSplit_PopulatesOutputFilesSorted(t *testing.T) {
	outDir := t.TempDir()
	for _, name := range []string{"split-track02.flac", "split-track01.flac"} {
		if err := os.WriteFile(filepath.Join(outDir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fake := &fakeRunner{exitCode: 0, stdout: "ok"}
	s := shntool.New(fake, "shntool")
	plan := domain.SplitPlan{
		CuePath: "/in/album.cue", ImagePath: "/in/album.flac", WorkDir: "/in",
	}
	res, err := s.Split(context.Background(), plan, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OutputFiles) != 2 {
		t.Fatalf("OutputFiles len=%d want 2", len(res.OutputFiles))
	}
	want := []string{
		filepath.Join(outDir, "split-track01.flac"),
		filepath.Join(outDir, "split-track02.flac"),
	}
	for i, p := range res.OutputFiles {
		abs, err := filepath.Abs(p)
		if err != nil {
			t.Fatal(err)
		}
		wantAbs, err := filepath.Abs(want[i])
		if err != nil {
			t.Fatal(err)
		}
		if abs != wantAbs {
			t.Fatalf("OutputFiles[%d]=%q want %q", i, abs, wantAbs)
		}
	}
}

func TestShntoolSplit_NoOutputFlacReturnsErrSplitFailed(t *testing.T) {
	fake := &fakeRunner{exitCode: 0, stdout: "ok"}
	s := shntool.New(fake, "shntool")
	plan := domain.SplitPlan{
		CuePath: "/in/album.cue", ImagePath: "/in/album.flac", WorkDir: "/in",
	}
	_, err := s.Split(context.Background(), plan, t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrSplitFailed) {
		t.Fatalf("errors.Is: got %v", err)
	}
}
