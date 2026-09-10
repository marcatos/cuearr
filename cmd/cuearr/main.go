package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/auth"
	"github.com/marcatos/cuearr/internal/adapters/config"
	fsadapter "github.com/marcatos/cuearr/internal/adapters/fs"
	httpapi "github.com/marcatos/cuearr/internal/adapters/http"
	"github.com/marcatos/cuearr/internal/adapters/logging"
	"github.com/marcatos/cuearr/internal/adapters/splitter/native"
	"github.com/marcatos/cuearr/internal/adapters/splitter/shntool"
	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cuearr <version|serve> [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "serve":
		os.Args = append(os.Args[:1], os.Args[2:]...)
		if err := runServe(); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func runServe() error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "configs/cuearr.yaml", "path to YAML config file")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	log := logging.New(cfg.LogLevel)
	slog.SetDefault(log)
	log.Info("cuearr serve starting", "version", version, "config", *configPath)

	if cfg.DataDir == "" {
		return fmt.Errorf("data_dir is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("data_dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "cuearr.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	log.Info("sqlite opened", "path", dbPath)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	settingsStore := store.Settings()
	if _, changed, err := auth.Bootstrap(ctx, settingsStore); err != nil {
		return fmt.Errorf("auth bootstrap: %w", err)
	} else if changed {
		log.Info("auth settings bootstrapped from environment")
	}
	defaultSettings := domain.Settings{
		WatchDirs: append([]string(nil), cfg.WatchDirs...),
		OutDir:    cfg.OutDir,
		InPlace:   cfg.InPlace,
		Engine:    cfg.Engine,
	}
	runtimeSettings, seeded, err := app.LoadRuntimeSettings(ctx, settingsStore, defaultSettings)
	if err != nil {
		return err
	}
	if seeded {
		log.Info("runtime settings seeded from YAML")
	}
	splitter, err := newSplitter(runtimeSettings.Engine)
	if err != nil {
		return err
	}
	runtimeCfg := app.NewRuntimeConfig(runtimeSettings, splitter)

	scanCtx := func(dir string) {
		settings, _ := runtimeCfg.Snapshot()
		_, created, scanErr := app.ScanDir(ctx, dir, store, settings.Engine, os.ReadFile, listDirEntries)
		if scanErr != nil {
			if errors.Is(scanErr, domain.ErrImageNotFound) {
				log.Debug("scan skipped", "dir", dir, "reason", scanErr.Error())
				return
			}
			log.Warn("scan dir failed", "dir", dir, "error", scanErr.Error())
			return
		}
		if created {
			log.Info("job enqueued from scan", "dir", dir)
		}
	}

	if len(runtimeSettings.WatchDirs) > 0 {
		log.Info("startup scan", "dirs", runtimeSettings.WatchDirs)
		start := time.Now()
		if err := app.ScanAll(runtimeSettings.WatchDirs, fsadapter.DefaultScanDepth, func(dir string) error {
			scanCtx(dir)
			return nil
		}); err != nil {
			log.Warn("startup scan failed", "error", err.Error())
		} else {
			log.Info("startup scan finished", "duration_ms", time.Since(start).Milliseconds())
		}
	} else {
		log.Warn("no watch_dirs configured; filesystem watcher disabled")
	}

	var wg sync.WaitGroup

	worker := &app.Worker{
		Store:   store,
		Runtime: runtimeCfg,
		Log:     log,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("worker exited", "error", err.Error())
		}
	}()

	watchers := newWatcherController(ctx, scanCtx, log)
	if err := watchers.Restart(runtimeSettings.WatchDirs); err != nil {
		return err
	}
	defer watchers.Stop()

	addr := cfg.HTTPAddr
	if addr == "" {
		addr = ":8787"
	}
	sessionSecret, err := auth.LoadOrCreateSessionSecret(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("session secret: %w", err)
	}
	api := httpapi.New(httpapi.Deps{
		Jobs:          store,
		Settings:      settingsStore,
		SessionSecret: sessionSecret,
		CookieSecure:  cfg.CookieSecure,
		RuntimeSettings: func() domain.Settings {
			settings, _ := runtimeCfg.Snapshot()
			return settings
		},
		ApplySettings: func(_ context.Context, next domain.Settings) error {
			nextSplitter, err := newSplitter(next.Engine)
			if err != nil {
				return err
			}
			current, _ := runtimeCfg.Snapshot()
			if !slices.Equal(current.WatchDirs, next.WatchDirs) {
				if err := watchers.Restart(next.WatchDirs); err != nil {
					return err
				}
			}
			runtimeCfg.Apply(next, nextSplitter)
			log.Info("runtime settings applied",
				"engine", next.Engine,
				"watch_dirs", len(next.WatchDirs),
				"in_place", next.InPlace,
			)
			return nil
		},
		CreateJob: func(c context.Context, path string) (domain.Job, bool, error) {
			settings, _ := runtimeCfg.Snapshot()
			return app.ScanDir(c, path, store, settings.Engine, os.ReadFile, listDirEntries)
		},
		ScanWatch: func(c context.Context) error {
			settings, _ := runtimeCfg.Snapshot()
			return app.ScanAll(settings.WatchDirs, fsadapter.DefaultScanDepth, func(dir string) error {
				scanCtx(dir)
				return nil
			})
		},
		CheckShntool: func(c context.Context) error {
			_, currentSplitter := runtimeCfg.Snapshot()
			return currentSplitter.Available(c)
		},
	})
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: api.Handler(),
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("http listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server exited", "error", err.Error())
			cancel()
		}
	}()

	log.Info("cuearr serve ready")
	<-ctx.Done()
	log.Info("cuearr serve shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	watchers.Stop()

	wg.Wait()
	log.Info("cuearr serve stopped")
	return nil
}

func newSplitter(engine string) (ports.Splitter, error) {
	switch engine {
	case "native":
		return native.New(), nil
	case "shntool", "":
		return shntool.New(&execRunner{}, "shntool"), nil
	default:
		return nil, fmt.Errorf("unknown engine %q", engine)
	}
}

type watcherController struct {
	rootCtx context.Context
	onDir   func(string)
	log     *slog.Logger
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newWatcherController(rootCtx context.Context, onDir func(string), log *slog.Logger) *watcherController {
	return &watcherController{rootCtx: rootCtx, onDir: onDir, log: log}
}

func (c *watcherController) Restart(dirs []string) error {
	start := time.Now()
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("watch directory %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("watch path %q is not a directory", dir)
		}
	}

	var watcher *fsadapter.Watcher
	var err error
	if len(dirs) > 0 {
		watcher, err = fsadapter.NewWatcher(dirs, func(dir string) error {
			c.onDir(dir)
			return nil
		}, c.log)
		if err != nil {
			return err
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
	if watcher != nil {
		watchCtx, cancel := context.WithCancel(c.rootCtx)
		c.cancel = cancel
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			if err := watcher.Start(watchCtx); err != nil && !errors.Is(err, context.Canceled) {
				c.log.Error("watcher exited", "error", err.Error())
			}
		}()
	}
	c.log.Info("watcher configuration applied",
		"watch_dirs", len(dirs),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (c *watcherController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *watcherController) stopLocked() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	c.wg.Wait()
	c.cancel = nil
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	exitCode = 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return outBuf.String(), errBuf.String(), -1, runErr
		}
	}
	return outBuf.String(), errBuf.String(), exitCode, nil
}

func listDirEntries(path string) ([]domain.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, domain.DirEntry{Name: e.Name()})
	}
	return out, nil
}
