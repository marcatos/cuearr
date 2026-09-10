package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestDetectAlbum_BasicCueAndFlac(t *testing.T) {
	cueBytes, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cue", "basic.cue"))
	if err != nil {
		t.Fatal(err)
	}
	dir := "/music/incoming/demo"

	listDir := func(string) ([]domain.DirEntry, error) {
		return []domain.DirEntry{
			{Name: "basic.cue"},
			{Name: "album.flac"},
		}, nil
	}
	readFile := func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "basic.cue") {
			return cueBytes, nil
		}
		return nil, os.ErrNotExist
	}

	plan, err := app.DetectAlbum(dir, readFile, listDir)
	if err != nil {
		t.Fatal(err)
	}
	if plan.WorkDir != dir {
		t.Fatalf("workDir=%q", plan.WorkDir)
	}
	if !strings.HasSuffix(plan.CuePath, "basic.cue") {
		t.Fatalf("cuePath=%q", plan.CuePath)
	}
	if !strings.HasSuffix(plan.ImagePath, "album.flac") {
		t.Fatalf("imagePath=%q", plan.ImagePath)
	}
	if len(plan.Tracks) != 2 {
		t.Fatalf("tracks=%d", len(plan.Tracks))
	}
	if plan.Fingerprint == "" {
		t.Fatal("expected fingerprint")
	}
}
