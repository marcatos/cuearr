package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/fs"
)

func TestWalkCueDirs_RespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "shallow.cue"), "cue")
	sub := filepath.Join(root, "a", "b", "c")
	mustMkdir(t, sub)
	mustWrite(t, filepath.Join(sub, "deep.cue"), "cue")

	dirs, err := fs.WalkCueDirs([]string{root}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 {
		t.Fatalf("dirs=%v want only shallow", dirs)
	}
	if dirs[0] != filepath.Clean(root) {
		t.Fatalf("dir=%q", dirs[0])
	}

	dirs, err = fs.WalkCueDirs([]string{root}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 {
		t.Fatalf("dirs=%v want shallow+deep", dirs)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
