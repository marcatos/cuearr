package fs

import (
	"context"
	"log/slog"
	"path/filepath"
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
		if err := fsw.Add(dir); err != nil {
			w.log.Error("watch add failed", "dir", dir, "err", err)
			return err
		}
		w.log.Info("watching directory", "dir", dir)
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
