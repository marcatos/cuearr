package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/fs"
)

func TestFileSafetyProbe_ReportsSizeWritableAndFreeSpace(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "album.flac")
	if err := os.WriteFile(imagePath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := fs.NewFileSafetyProbe()

	size, err := probe.StatSize(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if size != 5 {
		t.Fatalf("size=%d, want 5", size)
	}
	if err := probe.EnsureWritable(filepath.Join(dir, "output")); err != nil {
		t.Fatal(err)
	}
	available, err := probe.FreeSpace(dir)
	if err != nil {
		t.Fatal(err)
	}
	if available == 0 {
		t.Fatal("expected non-zero free space")
	}
}
