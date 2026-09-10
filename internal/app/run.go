package app

import (
	"context"
	"crypto/sha256"
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
	} else {
		targetOut = filepath.Join(outDir, albumOutputKey(job))
	}

	running := job
	if running.Status != domain.JobRunning {
		running.Status = domain.JobRunning
		running.StartedAt = time.Now().UTC()
		if err := store.Update(ctx, running); err != nil {
			return job, err
		}
	} else if running.StartedAt.IsZero() {
		running.StartedAt = time.Now().UTC()
		if err := store.Update(ctx, running); err != nil {
			return job, err
		}
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
