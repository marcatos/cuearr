package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

const pendingImportListLimit = 1000

type ImportService struct {
	Store  ports.JobStore
	Client ports.LidarrClient
	Clock  func() time.Time
	Log    *slog.Logger
}

// AfterSplitComplete updates import fields for a completed job using settings.
func (s ImportService) AfterSplitComplete(
	ctx context.Context,
	job domain.Job,
	settings domain.Settings,
) (result domain.Job, resultErr error) {
	start := time.Now()
	log := s.logger()
	log.Info("post-split import start", "job_id", job.ID, "import_status", job.ImportStatus)
	defer func() {
		log.Info("post-split import finished",
			"job_id", job.ID,
			"import_status", result.ImportStatus,
			"total_ms", time.Since(start).Milliseconds(),
			"ok", resultErr == nil,
		)
	}()

	if job.Status != domain.JobCompleted {
		return job, fmt.Errorf("job %q is not completed", job.ID)
	}

	if settings.InPlace {
		return s.skipIfPending(ctx, job, "in-place output")
	}
	if !settings.LidarrImportEnabled || s.Client == nil ||
		strings.TrimSpace(settings.LidarrURL) == "" ||
		strings.TrimSpace(settings.LidarrAPIKey) == "" {
		return s.skipIfPending(ctx, job, "Lidarr import disabled or not configured")
	}

	now := s.now()
	requested, err := domain.BeginImportRequest(job, now)
	if err != nil {
		return job, err
	}

	requestErr := s.Client.RequestImport(ctx, requested.OutDir)
	if requestErr != nil {
		result = domain.CompleteImportFail(requested, now, requestErr.Error())
	} else {
		result = domain.CompleteImportOK(requested, now)
	}
	if err := s.Store.Update(ctx, result); err != nil {
		return result, errors.Join(requestErr, fmt.Errorf("persist import result: %w", err))
	}
	if requestErr != nil {
		log.Warn("Lidarr import request failed",
			"job_id", job.ID,
			"output_dir", job.OutDir,
			"error", requestErr.Error(),
		)
		return result, requestErr
	}
	log.Info("Lidarr import request completed", "job_id", job.ID, "output_dir", job.OutDir)
	return result, nil
}

// RequestImportForJob is the manual/API path for completed jobs.
func (s ImportService) RequestImportForJob(
	ctx context.Context,
	jobID string,
	settings domain.Settings,
) (domain.Job, error) {
	start := time.Now()
	s.logger().Info("manual import request start", "job_id", jobID)
	job, err := s.Store.Get(ctx, jobID)
	if err != nil {
		s.logger().Error("manual import request lookup failed",
			"job_id", jobID,
			"total_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return domain.Job{}, err
	}
	result, err := s.AfterSplitComplete(ctx, job, settings)
	s.logger().Info("manual import request finished",
		"job_id", jobID,
		"import_status", result.ImportStatus,
		"total_ms", time.Since(start).Milliseconds(),
		"ok", err == nil,
	)
	return result, err
}

// PollPendingImports retries completed imports when the safety-net poller is enabled.
func (s ImportService) PollPendingImports(
	ctx context.Context,
	settings domain.Settings,
) (n int, resultErr error) {
	if settings.LidarrPollIntervalSec <= 0 {
		return 0, nil
	}

	start := time.Now()
	log := s.logger()
	log.Info("pending import poll start", "list_limit", pendingImportListLimit)
	defer func() {
		log.Info("pending import poll finished",
			"processed", n,
			"total_ms", time.Since(start).Milliseconds(),
			"ok", resultErr == nil,
		)
	}()

	jobs, err := s.Store.List(ctx, pendingImportListLimit)
	if err != nil {
		return 0, err
	}
	var requestErrors []error
	for _, job := range jobs {
		if job.Status != domain.JobCompleted || !importPending(job.ImportStatus) {
			continue
		}
		n++
		if _, err := s.RequestImportForJob(ctx, job.ID, settings); err != nil {
			requestErrors = append(requestErrors, fmt.Errorf("job %s: %w", job.ID, err))
		}
	}
	return n, errors.Join(requestErrors...)
}

func (s ImportService) skipIfPending(
	ctx context.Context,
	job domain.Job,
	reason string,
) (domain.Job, error) {
	if !importPending(job.ImportStatus) || job.ImportStatus == domain.ImportFailed {
		return job, nil
	}
	skipped := domain.SkipImport(job, reason)
	if err := s.Store.Update(ctx, skipped); err != nil {
		return skipped, fmt.Errorf("persist skipped import: %w", err)
	}
	s.logger().Info("Lidarr import skipped", "job_id", job.ID, "reason", reason)
	return skipped, nil
}

func (s ImportService) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func (s ImportService) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func importPending(status string) bool {
	return status == "" || status == domain.ImportNone || status == domain.ImportFailed
}
