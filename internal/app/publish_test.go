package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestPublishTracks_replaceDirPublishesAtomicallyViaSibling(t *testing.T) {
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

	var renames [][2]string
	origRename := osRename
	t.Cleanup(func() { osRename = origRename })
	osRename = func(oldpath, newpath string) error {
		renames = append(renames, [2]string{oldpath, newpath})
		return origRename(oldpath, newpath)
	}

	published, err := PublishTracks(staging, finalOut, "job-ok", []string{"01.flac", "02.flac"}, PublishReplaceDir)
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

	var sawTrackIntoFinal bool
	var sawDirPromote bool
	for _, pair := range renames {
		oldpath, newpath := pair[0], pair[1]
		if filepath.Dir(newpath) == finalOut && strings.HasSuffix(newpath, ".flac") {
			sawTrackIntoFinal = true
		}
		if oldpath != finalOut && filepath.Base(filepath.Dir(oldpath)) != filepath.Base(finalOut) &&
			newpath == finalOut {
			sawDirPromote = true
		}
		if newpath == finalOut {
			sawDirPromote = true
		}
		_ = oldpath
	}
	if sawTrackIntoFinal {
		t.Fatalf("tracks renamed directly into finalOut; renames=%v", renames)
	}
	if !sawDirPromote {
		t.Fatalf("expected sibling directory promote into finalOut; renames=%v", renames)
	}

	if err := CleanupStaging(staging); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging dir still exists: err=%v", err)
	}
}

func TestPublishTracks_replaceDirLeavesPreexistingFinalUntouchedOnFailure(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-fail")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	if err := os.MkdirAll(finalOut, 0o755); err != nil {
		t.Fatal(err)
	}
	preexisting := filepath.Join(finalOut, "01.flac")
	if err := os.WriteFile(preexisting, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("new-1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "02.flac"), []byte("new-2"), 0o644); err != nil {
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

	_, err = PublishTracks(staging, finalOut, "job-fail", []string{"01.flac", "02.flac"}, PublishReplaceDir)
	if err == nil {
		t.Fatal("expected error")
	}
	got, readErr := os.ReadFile(preexisting)
	if readErr != nil {
		t.Fatalf("pre-existing track missing after failed publish: %v", readErr)
	}
	if string(got) != "keep-me" {
		t.Fatalf("pre-existing track corrupted: %q", got)
	}
	if _, err := os.Stat(filepath.Join(finalOut, "02.flac")); !os.IsNotExist(err) {
		t.Fatalf("partial new track leaked into finalOut: err=%v", err)
	}
}

func TestPublishTracks_replaceDirSwapsExistingAlbumWithoutDeletingOldOnSwapFailure(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-swap")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	if err := os.MkdirAll(finalOut, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTrack := filepath.Join(finalOut, "old.flac")
	if err := os.WriteFile(oldTrack, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRename := osRename
	t.Cleanup(func() { osRename = origRename })
	osRename = func(oldpath, newpath string) error {
		// Fail when promoting the publishing dir onto finalOut after aside move.
		if newpath == finalOut && oldpath != finalOut && strings.Contains(oldpath, ".publishing-") {
			return errors.New("simulated swap promote failure")
		}
		return origRename(oldpath, newpath)
	}

	_, err = PublishTracks(staging, finalOut, "job-swap", []string{"01.flac"}, PublishReplaceDir)
	if err == nil {
		t.Fatal("expected error")
	}
	got, readErr := os.ReadFile(oldTrack)
	if readErr != nil {
		t.Fatalf("old album should be restored: %v", readErr)
	}
	if string(got) != "old" {
		t.Fatalf("old album content=%q", got)
	}
}

func TestPublishTracks_replaceDirRestoresInterruptedAsideBeforeRetry(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-recover")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	aside := filepath.Join(base, ".album.aside-job-recover")
	if err := os.MkdirAll(aside, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTrack := filepath.Join(aside, "old.flac")
	if err := os.WriteFile(oldTrack, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRename := osRename
	t.Cleanup(func() { osRename = origRename })
	osRename = func(oldpath, newpath string) error {
		if newpath == finalOut && strings.Contains(oldpath, ".publishing-") {
			return errors.New("simulated retry promote failure")
		}
		return origRename(oldpath, newpath)
	}

	_, err = PublishTracks(staging, finalOut, "job-recover", []string{"01.flac"}, PublishReplaceDir)
	if err == nil {
		t.Fatal("expected error")
	}
	got, readErr := os.ReadFile(filepath.Join(finalOut, "old.flac"))
	if readErr != nil {
		t.Fatalf("interrupted old album was not restored: %v", readErr)
	}
	if string(got) != "old" {
		t.Fatalf("old album content=%q", got)
	}
}

func TestPublishTracks_mergeIntoRefusesOverwriteOfExistingTrack(t *testing.T) {
	base := t.TempDir()
	staging, err := PrepareStaging(base, "job-merge")
	if err != nil {
		t.Fatal(err)
	}
	finalOut := filepath.Join(base, "album")
	if err := os.MkdirAll(finalOut, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(finalOut, "01.flac")
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = PublishTracks(staging, finalOut, "job-merge", []string{"01.flac"}, PublishMergeInto)
	if err == nil {
		t.Fatal("expected overwrite refusal")
	}
	got, readErr := os.ReadFile(existing)
	if readErr != nil || string(got) != "keep" {
		t.Fatalf("existing=%q err=%v", got, readErr)
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

	_, err = PublishTracks(staging, finalOut, "job-exdev", []string{"01.flac"}, PublishReplaceDir)
	if !errors.Is(err, ErrCrossDeviceRename) {
		t.Fatalf("err=%v want ErrCrossDeviceRename", err)
	}
}
