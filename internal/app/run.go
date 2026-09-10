package app

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

func RunJob(ctx context.Context, store ports.JobStore, splitter ports.Splitter, job domain.Job, outDir string, inPlace bool) (domain.Job, error) {
	log := slog.Default()
	log.Info("run job start", "job_id", job.ID, "engine", splitter.Name(), "cue_path", job.CuePath)

	targetOut := outDir
	if inPlace {
		targetOut = filepath.Dir(job.ImagePath)
	}

	running := job
	running.Status = domain.JobRunning
	running.StartedAt = time.Now().UTC()
	if err := store.Update(ctx, running); err != nil {
		return job, err
	}

	plan := domain.SplitPlan{
		CuePath:   job.CuePath,
		ImagePath: job.ImagePath,
		WorkDir:   filepath.Dir(job.ImagePath),
	}

	splitStart := time.Now()
	result, splitErr := splitter.Split(ctx, plan, targetOut)
	splitMs := time.Since(splitStart).Milliseconds()
	log.Info("run job split finished", "job_id", job.ID, "duration_ms", splitMs, "ok", splitErr == nil)

	finished := running
	finished.OutDir = targetOut
	finished.FinishedAt = time.Now().UTC()

	if splitErr != nil {
		finished.Status = domain.JobFailed
		finished.Error = splitErr.Error()
		if result.Log != "" {
			finished.Log = result.Log
		}
		if err := store.Update(ctx, finished); err != nil {
			return finished, err
		}
		log.Warn("run job failed", "job_id", job.ID, "error", splitErr.Error())
		return finished, splitErr
	}

	finished.Status = domain.JobCompleted
	finished.Log = buildJobLog(result)
	if err := store.Update(ctx, finished); err != nil {
		return finished, err
	}

	log.Info("run job completed",
		"job_id", job.ID,
		"output_files", len(result.OutputFiles),
		"split_duration_ms", splitMs,
	)
	return finished, nil
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
