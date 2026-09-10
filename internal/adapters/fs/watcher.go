package fs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceInterval = 2 * time.Second

type Watcher struct {
	dirs  []string
	onDir func(dir string) error
	log   *slog.Logger
	newFS func() (*fsnotify.Watcher, error)
	wait  time.Duration
}

func NewWatcher(dirs []string, onDir func(dir string) error, log *slog.Logger) (*Watcher, error) {
	if log == nil {
		log = slog.Default()
	}
	if onDir == nil {
		return nil, errOnDirRequired
	}
	cleaned := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d == "" {
			continue
		}
		cleaned = append(cleaned, filepath.Clean(d))
	}
	if len(cleaned) == 0 {
		return nil, errNoWatchDirs
	}
	return &Watcher{
		dirs:  cleaned,
		onDir: onDir,
		log:   log,
		newFS: fsnotify.NewWatcher,
		wait:  debounceInterval,
	}, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	fsw, err := w.newFS()
	if err != nil {
		return err
	}
	defer fsw.Close()

	for _, dir := range w.dirs {
		if err := addWatchTree(fsw, w.log, dir, 0, DefaultScanDepth); err != nil {
			w.log.Error("watch add failed", "dir", dir, "err", err)
			return err
		}
	}

	debounce := newDebouncer(w.wait, func(dir string) {
		w.log.Info("album directory changed", "dir", dir)
		if err := w.onDir(dir); err != nil {
			w.log.Error("onDir failed", "dir", dir, "err", err)
		}
	})
	defer debounce.stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("filesystem watcher stopped")
			return ctx.Err()
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			w.log.Warn("fsnotify error", "err", err)
		case ev, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					depth, withinRoots := watchDepth(w.dirs, ev.Name)
					if !withinRoots || depth > DefaultScanDepth {
						continue
					}
					if err := addWatchTree(fsw, w.log, ev.Name, depth, DefaultScanDepth); err != nil {
						w.log.Warn("watch add failed for new directory", "dir", ev.Name, "err", err)
					} else {
						debounce.schedule(filepath.Clean(ev.Name))
					}
					continue
				}
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			base := filepath.Base(ev.Name)
			if !IsWatchTarget(base) {
				continue
			}
			dir := ResolveAlbumDir(ev.Name)
			debounce.schedule(dir)
		}
	}
}

func watchDepth(roots []string, path string) (int, bool) {
	for _, root := range roots {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return len(strings.Split(rel, string(filepath.Separator))), true
	}
	return 0, false
}

type debouncer struct {
	mu     sync.Mutex
	wait   time.Duration
	timers map[string]*time.Timer
	fn     func(dir string)
}

func newDebouncer(wait time.Duration, fn func(dir string)) *debouncer {
	return &debouncer{
		wait:   wait,
		timers: make(map[string]*time.Timer),
		fn:     fn,
	}
}

func (d *debouncer) schedule(dir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[dir]; ok {
		t.Stop()
	}
	dirKey := dir
	d.timers[dirKey] = time.AfterFunc(d.wait, func() {
		d.mu.Lock()
		delete(d.timers, dirKey)
		d.mu.Unlock()
		d.fn(dirKey)
	})
}

func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, t := range d.timers {
		t.Stop()
	}
}

func addWatchTree(fsw *fsnotify.Watcher, log *slog.Logger, dir string, depth, maxDepth int) error {
	if err := fsw.Add(dir); err != nil {
		return err
	}
	log.Info("watching directory", "dir", dir)
	if depth >= maxDepth {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		if err := addWatchTree(fsw, log, child, depth+1, maxDepth); err != nil {
			return err
		}
	}
	return nil
}
