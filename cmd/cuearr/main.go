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

	splitter, err := newSplitter(cfg.Engine)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	scanCtx := func(dir string) {
		_, created, scanErr := app.ScanDir(ctx, dir, store, cfg.Engine, os.ReadFile, listDirEntries)
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

	if len(cfg.WatchDirs) > 0 {
		log.Info("startup scan", "dirs", cfg.WatchDirs)
		start := time.Now()
		if err := app.ScanAll(cfg.WatchDirs, fsadapter.DefaultScanDepth, func(dir string) error {
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
		Store:    store,
		Splitter: splitter,
		OutDir:   cfg.OutDir,
		InPlace:  cfg.InPlace,
		Log:      log,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("worker exited", "error", err.Error())
		}
	}()

	if len(cfg.WatchDirs) > 0 {
		watcher, wErr := fsadapter.NewWatcher(cfg.WatchDirs, func(dir string) error {
			scanCtx(dir)
			return nil
		}, log)
		if wErr != nil {
			return wErr
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := watcher.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("watcher exited", "error", err.Error())
			}
		}()
	}

	addr := cfg.HTTPAddr
	if addr == "" {
		addr = ":8787"
	}
	settingsStore := store.Settings()
	if _, changed, err := auth.Bootstrap(ctx, settingsStore); err != nil {
		return fmt.Errorf("auth bootstrap: %w", err)
	} else if changed {
		log.Info("auth settings bootstrapped from environment")
	}
	sessionSecret, err := auth.NewSessionSecret()
	if err != nil {
		return err
	}
	api := httpapi.New(httpapi.Deps{
		Jobs:          store,
		Settings:      settingsStore,
		SessionSecret: sessionSecret,
		Engine:    cfg.Engine,
		WatchDirs: cfg.WatchDirs,
		CreateJob: func(c context.Context, path string) (domain.Job, bool, error) {
			return app.ScanDir(c, path, store, cfg.Engine, os.ReadFile, listDirEntries)
		},
		ScanWatch: func(c context.Context) error {
			return app.ScanAll(cfg.WatchDirs, fsadapter.DefaultScanDepth, func(dir string) error {
				scanCtx(dir)
				return nil
			})
		},
		CheckShntool: func(c context.Context) error {
			return splitter.Available(c)
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
