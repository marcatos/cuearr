package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcatos/cuearr/internal/app"
)

func TestScanAll_FindsCueDirs(t *testing.T) {
	root := t.TempDir()
	album := filepath.Join(root, "incoming", "Artist - Album")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album, "album.cue"), []byte("cue"), 0o644); err != nil {
		t.Fatal(err)
	}

	var found []string
	err := app.ScanAll([]string{filepath.Join(root, "incoming")}, 3, func(dir string) error {
		found = append(found, dir)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != filepath.Clean(album) {
		t.Fatalf("found=%v", found)
	}
}

func TestScanAll_DefaultMaxDepthWhenZero(t *testing.T) {
	root := t.TempDir()
	album := filepath.Join(root, "a", "b", "album")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album, "disc.cue"), []byte("cue"), 0o644); err != nil {
		t.Fatal(err)
	}

	var found []string
	err := app.ScanAll([]string{root}, 0, func(dir string) error {
		found = append(found, dir)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != filepath.Clean(album) {
		t.Fatalf("found=%v want %q", found, filepath.Clean(album))
	}
}
