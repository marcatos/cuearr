package fs_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/fs"
)

func TestWatcher_DebouncesCueAndFlacWrites(t *testing.T) {
	root := t.TempDir()
	cuePath := filepath.Join(root, "album.cue")
	flacPath := filepath.Join(root, "album.flac")

	var mu sync.Mutex
	var dirs []string
	onDir := func(dir string) error {
		mu.Lock()
		dirs = append(dirs, dir)
		mu.Unlock()
		return nil
	}

	w, err := fs.NewWatcher([]string{root}, onDir, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Start(ctx) }()

	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(cuePath, []byte("FILE \"album.flac\" WAVE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(flacPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(dirs)
		mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for onDir")
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(2500 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(dirs) != 1 {
		t.Fatalf("onDir calls=%d dirs=%v", len(dirs), dirs)
	}
	if dirs[0] != filepath.Clean(root) {
		t.Fatalf("onDir dir=%q want %q", dirs[0], root)
	}
}

func TestWatcher_NestedAlbumDirTriggersOnDir(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "nested", "album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cuePath := filepath.Join(albumDir, "album.cue")
	flacPath := filepath.Join(albumDir, "album.flac")

	var mu sync.Mutex
	var dirs []string
	onDir := func(dir string) error {
		mu.Lock()
		dirs = append(dirs, dir)
		mu.Unlock()
		return nil
	}

	w, err := fs.NewWatcher([]string{root}, onDir, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Start(ctx) }()

	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(cuePath, []byte("FILE \"album.flac\" WAVE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(flacPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	wantDir := filepath.Clean(albumDir)
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(dirs)
		mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for onDir")
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(dirs) != 1 {
		t.Fatalf("onDir calls=%d dirs=%v", len(dirs), dirs)
	}
	if dirs[0] != wantDir {
		t.Fatalf("onDir dir=%q want %q", dirs[0], wantDir)
	}
}
