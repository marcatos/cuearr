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
		if err := addWatchTree(ctx, fsw, w.log, dir, 0, DefaultScanDepth); err != nil {
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
					if err := addWatchTree(ctx, fsw, w.log, ev.Name, depth, DefaultScanDepth); err != nil {
						w.log.Warn("watch add failed for new directory", "dir", ev.Name, "err", err)
					} else {
						scheduleCueDirsUnder(ctx, debounce, ev.Name, DefaultScanDepth-depth)
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

func scheduleCueDirsUnder(ctx context.Context, debounce *debouncer, dir string, maxDepth int) {
	dir = filepath.Clean(dir)
	if maxDepth < 0 {
		maxDepth = 0
	}
	seen := make(map[string]struct{})
	var cueDirs []string
	if err := walkCueDirs(ctx, dir, 0, maxDepth, seen, &cueDirs); err != nil {
		debounce.schedule(dir)
		return
	}
	if len(cueDirs) == 0 {
		debounce.schedule(dir)
		return
	}
	for _, d := range cueDirs {
		debounce.schedule(d)
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
	mu      sync.Mutex
	wait    time.Duration
	timers  map[string]*time.Timer
	fn      func(dir string)
	stopped bool
	running sync.WaitGroup
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
	if d.stopped {
		return
	}
	if t, ok := d.timers[dir]; ok {
		t.Stop()
	}
	dirKey := dir
	var timer *time.Timer
	timer = time.AfterFunc(d.wait, func() {
		d.mu.Lock()
		if d.stopped || d.timers[dirKey] != timer {
			d.mu.Unlock()
			return
		}
		delete(d.timers, dirKey)
		d.running.Add(1)
		d.mu.Unlock()
		defer d.running.Done()
		d.fn(dirKey)
	})
	d.timers[dirKey] = timer
}

func (d *debouncer) stop() {
	d.mu.Lock()
	d.stopped = true
	for _, t := range d.timers {
		t.Stop()
	}
	clear(d.timers)
	d.mu.Unlock()
	d.running.Wait()
}

func addWatchTree(ctx context.Context, fsw *fsnotify.Watcher, log *slog.Logger, dir string, depth, maxDepth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
		if err := addWatchTree(ctx, fsw, log, child, depth+1, maxDepth); err != nil {
			return err
		}
	}
	return nil
}
