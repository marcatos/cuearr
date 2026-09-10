package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

func (r JobRunner) RunJob(ctx context.Context, job domain.Job, outDir string, inPlace bool) (resultJob domain.Job, resultErr error) {
	start := time.Now()
	log := r.logger()
	log.Info("run job start", "job_id", job.ID, "engine", r.Splitter.Name(), "cue_path", job.CuePath)
	defer func() {
		log.Info("run job finished",
			"job_id", job.ID,
			"status", resultJob.Status,
			"total_ms", time.Since(start).Milliseconds(),
			"ok", resultErr == nil,
		)
	}()

	targetOut := outDir
	if inPlace {
		targetOut = filepath.Dir(job.ImagePath)
	} else {
		targetOut = filepath.Join(outDir, albumOutputKey(job))
	}

	running := job
	if running.Status != domain.JobRunning {
		running.Status = domain.JobRunning
		running.StartedAt = time.Now().UTC()
		if err := r.Store.Update(ctx, running); err != nil {
			return job, err
		}
	} else if running.StartedAt.IsZero() {
		running.StartedAt = time.Now().UTC()
		if err := r.Store.Update(ctx, running); err != nil {
			return job, err
		}
	}

	plan := domain.SplitPlan{
		CuePath:   job.CuePath,
		ImagePath: job.ImagePath,
		WorkDir:   filepath.Dir(job.ImagePath),
	}

	splitStart := time.Now()
	result, splitErr := r.Splitter.Split(ctx, plan, targetOut)
	splitMs := time.Since(splitStart).Milliseconds()
	log.Info("run job split finished", "job_id", job.ID, "duration_ms", splitMs, "ok", splitErr == nil)

	finished := running
	finished.OutDir = targetOut

	if splitErr != nil {
		finished.FinishedAt = time.Now().UTC()
		finished.Status = domain.JobFailed
		finished.Error = splitErr.Error()
		if result.Log != "" {
			finished.Log = result.Log
		}
		if err := r.Store.Update(ctx, finished); err != nil {
			return finished, err
		}
		log.Warn("run job failed", "job_id", job.ID, "error", splitErr.Error())
		return finished, splitErr
	}

	if len(result.OutputFiles) == 0 {
		return r.failJob(ctx, finished, result, errors.New("split produced no output files"), log)
	}

	imageInfo, err := r.Inspector.Inspect(ctx, job.ImagePath)
	if err != nil {
		return r.failJob(ctx, finished, result, fmt.Errorf("inspect source image %q: %w", job.ImagePath, err), log)
	}
	if err := r.VerifySplit(ctx, job.CuePath, result.OutputFiles, imageInfo.Duration); err != nil {
		return r.failJob(ctx, finished, result, err, log)
	}

	finished.FinishedAt = time.Now().UTC()
	finished.Status = domain.JobCompleted
	finished.Log = buildJobLog(result)
	if err := r.Store.Update(ctx, finished); err != nil {
		return finished, err
	}

	log.Info("run job completed",
		"job_id", job.ID,
		"output_files", len(result.OutputFiles),
		"split_duration_ms", splitMs,
	)
	return finished, nil
}

func (r JobRunner) failJob(
	ctx context.Context,
	finished domain.Job,
	result ports.SplitResult,
	runErr error,
	log *slog.Logger,
) (domain.Job, error) {
	finished.FinishedAt = time.Now().UTC()
	finished.Status = domain.JobFailed
	finished.Error = runErr.Error()
	finished.Log = buildJobLog(result)
	if err := r.Store.Update(ctx, finished); err != nil {
		return finished, err
	}
	log.Warn("run job failed", "job_id", finished.ID, "error", runErr.Error())
	return finished, runErr
}

func albumOutputKey(job domain.Job) string {
	key := strings.TrimSpace(job.Fingerprint)
	if key != "" {
		safe := true
		for _, r := range key {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
				(r < '0' || r > '9') && r != '-' && r != '_' {
				safe = false
				break
			}
		}
		if safe {
			return key
		}
	}
	sum := sha256.Sum256([]byte(job.CuePath + "\x00" + job.ImagePath + "\x00" + key))
	return fmt.Sprintf("album-%x", sum[:8])
}

func buildJobLog(result ports.SplitResult) string {
	var b strings.Builder
	if result.Log != "" {
		b.WriteString(result.Log)
	}
	if len(result.OutputFiles) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("output files:\n")
		for _, f := range result.OutputFiles {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}
	return strings.TrimSpace(b.String())
}
