package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type Worker struct {
	Store    ports.JobStore
	Splitter ports.Splitter
	Runtime  *RuntimeConfig
	OutDir   string
	InPlace  bool
	Log      *slog.Logger
	Interval time.Duration
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
	job, ok, err := w.nextQueued(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	log := w.logger()
	log.Info("worker picked job", "job_id", job.ID)

	splitter, outDir, inPlace := w.Splitter, w.OutDir, w.InPlace
	if w.Runtime != nil {
		settings, currentSplitter := w.Runtime.Snapshot()
		splitter, outDir, inPlace = currentSplitter, settings.OutDir, settings.InPlace
	}
	_, runErr := RunJob(ctx, w.Store, splitter, job, outDir, inPlace)
	totalMs := time.Since(start).Milliseconds()
	if runErr != nil {
		log.Warn("worker job finished with error", "job_id", job.ID, "total_ms", totalMs, "error", runErr.Error())
		return nil
	}
	log.Info("worker job finished", "job_id", job.ID, "total_ms", totalMs)
	return nil
}

func (w *Worker) nextQueued(ctx context.Context) (domain.Job, bool, error) {
	jobs, err := w.Store.List(ctx, 200)
	if err != nil {
		return domain.Job{}, false, err
	}
	var (
		found domain.Job
		ok    bool
	)
	for _, j := range jobs {
		if j.Status != domain.JobQueued {
			continue
		}
		if !ok || j.CreatedAt.Before(found.CreatedAt) {
			found = j
			ok = true
		}
	}
	return found, ok, nil
}
