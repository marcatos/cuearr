package app

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStagingDir(t *testing.T) {
	got := StagingDir(filepath.Join("base", "out"), "job-1")
	want := filepath.Join("base", "out", ".cuearr-staging", "job-1")
	if got != want {
		t.Fatalf("StagingDir()=%q want %q", got, want)
	}
}

func TestPrepareStaging_createsAndWipes(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-a")
	if err != nil {
		t.Fatal(err)
	}
	if staging != StagingDir(base, "job-a") {
		t.Fatalf("staging=%q", staging)
	}
	stale := filepath.Join(staging, "old.flac")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	staging2, err := PrepareStaging(base, "job-a")
	if err != nil {
		t.Fatal(err)
	}
	if staging2 != staging {
		t.Fatalf("staging2=%q", staging2)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file still present: err=%v", err)
	}
}

func TestPublishTracks_successMovesToFinalAndEmptiesStaging(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-ok")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	for _, name := range []string{"01.flac", "02.flac"} {
		if err := os.WriteFile(filepath.Join(staging, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	published, err := PublishTracks(staging, finalOut, []string{"01.flac", "02.flac"})
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 {
		t.Fatalf("published=%v", published)
	}
	for _, name := range []string{"01.flac", "02.flac"} {
		finalPath := filepath.Join(finalOut, name)
		if _, err := os.Stat(finalPath); err != nil {
			t.Fatalf("final %q: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(staging, name)); !os.IsNotExist(err) {
			t.Fatalf("staging still has %q: err=%v", name, err)
		}
	}

	if err := CleanupStaging(staging); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging dir still exists: err=%v", err)
	}
}

func TestPublishTracks_midFailureRollsBackPartialFinal(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-fail")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "02.flac"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRename := osRename
	t.Cleanup(func() { osRename = origRename })
	calls := 0
	osRename = func(oldpath, newpath string) error {
		calls++
		if calls == 2 {
			return errors.New("simulated rename failure")
		}
		return origRename(oldpath, newpath)
	}

	_, err = PublishTracks(staging, finalOut, []string{"01.flac", "02.flac"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(finalOut, "01.flac")); !os.IsNotExist(err) {
		t.Fatalf("partial publish not rolled back in finalOut: err=%v", err)
	}
	// First file was already renamed out of staging; rollback only removes partial finalOut files.
	if _, err := os.Stat(filepath.Join(staging, "01.flac")); !os.IsNotExist(err) {
		t.Fatalf("renamed track should not remain in staging: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(staging, "02.flac")); err != nil {
		t.Fatalf("unpublished track should remain in staging: %v", err)
	}
}

func TestPublishTracks_crossDeviceIsPermanentFailure(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-exdev")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRename := osRename
	t.Cleanup(func() { osRename = origRename })
	osRename = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EXDEV}
	}

	_, err = PublishTracks(staging, finalOut, []string{"01.flac"})
	if !errors.Is(err, ErrCrossDeviceRename) {
		t.Fatalf("err=%v want ErrCrossDeviceRename", err)
	}
}
