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

	stagingBase := outDir
	if inPlace {
		stagingBase = filepath.Dir(job.ImagePath)
	}
	finalOut := filepath.Join(stagingBase, albumOutputKey(job))

	running := domain.BeginAttempt(job, time.Now().UTC())
	if err := r.Store.Update(ctx, running); err != nil {
		return job, err
	}
	log.Info("run job attempt started",
		"job_id", job.ID,
		"attempt", running.AttemptCount,
	)

	preflightStart := time.Now()
	if r.Preflight == nil {
		finished := running
		finished.OutDir = finalOut
		return r.failJob(ctx, finished, ports.SplitResult{}, ErrPreflightUnavailable, log)
	}
	if err := r.Preflight.Check(ctx, job.ImagePath, stagingBase); err != nil {
		finished := running
		finished.OutDir = finalOut
		return r.failJob(ctx, finished, ports.SplitResult{}, err, log)
	}
	log.Info("run job preflight passed",
		"job_id", job.ID,
		"duration_ms", time.Since(preflightStart).Milliseconds(),
	)

	stagingStart := time.Now()
	staging, err := PrepareStaging(stagingBase, job.ID)
	if err != nil {
		finished := running
		finished.OutDir = finalOut
		return r.failJob(ctx, finished, ports.SplitResult{}, err, log)
	}
	log.Info("run job staging prepared",
		"job_id", job.ID,
		"staging", staging,
		"duration_ms", time.Since(stagingStart).Milliseconds(),
	)
	cleanup := func(runErr error) error {
		cleanupStart := time.Now()
		cleanupErr := CleanupStaging(staging)
		log.Info("run job staging cleanup finished",
			"job_id", job.ID,
			"duration_ms", time.Since(cleanupStart).Milliseconds(),
			"ok", cleanupErr == nil,
		)
		if cleanupErr != nil {
			return errors.Join(runErr, cleanupErr)
		}
		return runErr
	}

	plan := domain.SplitPlan{
		CuePath:   job.CuePath,
		ImagePath: job.ImagePath,
		WorkDir:   filepath.Dir(job.ImagePath),
	}

	splitStart := time.Now()
	result, splitErr := r.Splitter.Split(ctx, plan, staging)
	splitMs := time.Since(splitStart).Milliseconds()
	log.Info("run job split finished", "job_id", job.ID, "duration_ms", splitMs, "ok", splitErr == nil)

	finished := running
	finished.OutDir = finalOut

	if splitErr != nil {
		splitErr = cleanup(splitErr)
		return r.failJob(ctx, finished, result, splitErr, log)
	}

	if len(result.OutputFiles) == 0 {
		runErr := cleanup(errors.New("split produced no output files"))
		return r.failJob(ctx, finished, result, runErr, log)
	}

	imageInfo, err := r.inspectSource(ctx, job.ImagePath)
	if err != nil {
		runErr := cleanup(fmt.Errorf("inspect source image %q: %w", job.ImagePath, err))
		return r.failJob(ctx, finished, result, runErr, log)
	}
	if err := r.VerifySplit(ctx, job.CuePath, result.OutputFiles, imageInfo.Duration); err != nil {
		runErr := cleanup(err)
		return r.failJob(ctx, finished, result, runErr, log)
	}

	stagedFiles := result.OutputFiles
	result.OutputFiles = publishedPaths(finalOut, stagedFiles)
	finished.FinishedAt = time.Now().UTC()
	finished.Status = domain.JobCompleted
	finished.Log = buildJobLog(result)
	if err := r.Store.Update(ctx, finished); err != nil {
		persistErr := fmt.Errorf("persist completed job: %w", err)
		runErr := cleanup(persistErr)
		return r.failJob(ctx, finished, result, runErr, log)
	}

	published, err := PublishTracks(staging, finalOut, job.ID, stagedFiles, PublishReplaceDir)
	if err != nil {
		runErr := cleanup(err)
		return r.failJob(ctx, finished, result, runErr, log)
	}
	result.OutputFiles = published
	if err := cleanup(nil); err != nil {
		return r.failJob(ctx, finished, result, err, log)
	}

	if r.Importer != nil {
		settings := domain.Settings{}
		if r.Settings != nil {
			settings = r.Settings()
		}
		imported, importErr := r.Importer.AfterSplitComplete(ctx, finished, settings)
		finished = imported
		if importErr != nil {
			log.Warn("post-split import failed without failing completed job",
				"job_id", job.ID,
				"import_status", finished.ImportStatus,
				"error", importErr.Error(),
			)
		}
	}

	log.Info("run job completed",
		"job_id", job.ID,
		"output_files", len(result.OutputFiles),
		"split_duration_ms", splitMs,
	)
	return finished, nil
}

func (r JobRunner) inspectSource(ctx context.Context, path string) (ports.FLACInfo, error) {
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		if r.WAVInspector == nil {
			return ports.FLACInfo{}, errors.New("WAV source inspector is unavailable")
		}
		return r.WAVInspector.Inspect(ctx, path)
	}
	return r.Inspector.Inspect(ctx, path)
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
