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
	statFile := func(path string) (domain.FileStat, error) {
		if strings.HasSuffix(path, "album.flac") {
			return domain.FileStat{Size: 4096, ModTimeUnixNano: 1700000000000000000}, nil
		}
		return domain.FileStat{}, os.ErrNotExist
	}
	hashFile := func(path string) (string, error) {
		if strings.HasSuffix(path, "album.flac") {
			return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", nil
		}
		return "", os.ErrNotExist
	}

	plan, err := app.DetectAlbum(dir, readFile, listDir, statFile, hashFile)
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
	wantFP := domain.Fingerprint(plan.CuePath, plan.ImagePath, cueBytes, domain.ImageIdentity{
		Size:            4096,
		ModTimeUnixNano: 1700000000000000000,
		ContentSHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	})
	if plan.Fingerprint != wantFP {
		t.Fatalf("fingerprint=%q want=%q", plan.Fingerprint, wantFP)
	}
}
