package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type Worker struct {
	Store     ports.JobStore
	Splitter  ports.Splitter
	Inspector ports.FLACInspector
	Tagger    ports.FLACTagger
	Preflight ports.JobPreflight
	ReadFile  func(string) ([]byte, error)
	Runtime   *RuntimeConfig
	OutDir    string
	InPlace   bool
	Log       *slog.Logger
	Interval  time.Duration
}

func (w *Worker) logger() *slog.Logger {
	if w.Log != nil {
		return w.Log
	}
	return slog.Default()
}

func (w *Worker) pollInterval() time.Duration {
	if w.Interval > 0 {
		return w.Interval
	}
	return 2 * time.Second
}

func (w *Worker) Run(ctx context.Context) error {
	log := w.logger()
	log.Info("worker start", "interval", w.pollInterval().String())
	defer log.Info("worker stopped")
	if recoverer, ok := w.Store.(ports.RunningJobRecoverer); ok {
		recovered, err := recoverer.RecoverRunning(ctx)
		if err != nil {
			return err
		}
		log.Info("worker recovery completed", "requeued_jobs", recovered)
	}

	ticker := time.NewTicker(w.pollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.runOnce(ctx); err != nil {
				log.Error("worker tick failed", "error", err)
			}
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) error {
	start := time.Now()
	job, err := w.Store.ClaimNextQueued(ctx, time.Now().UTC())
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	log := w.logger()
	log.Info("worker claimed job", "job_id", job.ID)

	splitter, outDir, inPlace := w.Splitter, w.OutDir, w.InPlace
	maxAttempts := (domain.Settings{}).MaxAttempts()
	if w.Runtime != nil {
		settings, currentSplitter := w.Runtime.Snapshot()
		splitter, outDir, inPlace = currentSplitter, settings.OutDir, settings.InPlace
		maxAttempts = settings.MaxAttempts()
	}
	runner := JobRunner{
		Store: w.Store, Splitter: splitter, Inspector: w.Inspector,
		Tagger: w.Tagger, Preflight: w.Preflight, ReadFile: w.ReadFile, Log: log,
	}
	finished, runErr := runner.RunJob(ctx, job, outDir, inPlace)
	totalMs := time.Since(start).Milliseconds()
	if runErr != nil {
		finished = domain.ApplyAttemptFailure(finished, runErr.Error(), time.Now().UTC(), maxAttempts)
		if err := w.Store.Update(ctx, finished); err != nil {
			return err
		}
		log.Warn("worker job attempt failed",
			"job_id", job.ID,
			"attempt", finished.AttemptCount,
			"max_attempts", maxAttempts,
			"requeued", finished.Status == domain.JobQueued,
			"total_ms", totalMs,
			"error", runErr.Error(),
		)
		return nil
	}
	log.Info("worker job finished", "job_id", job.ID, "total_ms", totalMs)
	return nil
}
