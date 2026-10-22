package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

var ErrPublicationPending = errors.New("publication needs recovery")

const publicationFile = ".cuearr-publication.json"

type publicationManifest struct {
	JobID string   `json:"job_id"`
	Files []string `json:"files"`
}

// recoverPublications settles every job left in the publishing state. With
// isolate unset it stops at the first failure, which keeps startup fail-closed.
// With isolate set, a job whose filesystem state cannot be recovered is marked
// failed and the remaining jobs are still processed, so one bad publication
// never stalls the queue; store errors are joined and returned after the loop.
func (w *Worker) recoverPublications(ctx context.Context, isolate bool) error {
	lister, ok := w.Store.(ports.PublishingJobLister)
	if !ok {
		return nil
	}
	jobs, err := lister.ListPublishing(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, job := range jobs {
		err := w.recoverPublishingJob(ctx, job, isolate)
		if err == nil {
			continue
		}
		if !isolate {
			return err
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (w *Worker) recoverPublishingJob(ctx context.Context, job domain.Job, quarantine bool) error {
	complete, err := recoverPublication(job)
	if err != nil {
		err = fmt.Errorf("recover publication %s: %w", job.ID, err)
		if !quarantine {
			return err
		}
		w.logger().Error("publication recovery failed; marking job failed", "job_id", job.ID, "error", err)
		job.Status = domain.JobFailed
		job.FinishedAt = time.Now().UTC()
		job.Error = err.Error()
		return w.Store.Update(ctx, job)
	}
	if complete {
		job.Status = domain.JobCompleted
		job.FinishedAt = time.Now().UTC()
	} else {
		job.Status = domain.JobQueued
		job.StartedAt = time.Time{}
	}
	if err := w.Store.Update(ctx, job); err != nil {
		return err
	}
	if complete && w.Importer != nil {
		settings := domain.Settings{}
		if w.Runtime != nil {
			settings, _ = w.Runtime.Snapshot()
		}
		if _, err := w.Importer.AfterSplitComplete(ctx, job, settings); err != nil {
			w.logger().Warn("recovered publication import failed", "job_id", job.ID, "error", err)
		}
	}
	return nil
}

func recoverPublication(job domain.Job) (bool, error) {
	if job.OutDir == "" {
		return false, errors.New("publication output is empty")
	}
	parent, base := filepath.Dir(job.OutDir), filepath.Base(job.OutDir)
	aside := filepath.Join(parent, "."+base+".aside-"+job.ID)
	publishing := filepath.Join(parent, "."+base+".publishing-"+job.ID)
	data, err := os.ReadFile(filepath.Join(job.OutDir, publicationFile))
	complete := false
	if err == nil {
		var manifest publicationManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return false, err
		}
		if manifest.JobID == job.ID && len(manifest.Files) > 0 {
			for _, name := range manifest.Files {
				if filepath.Base(name) != name {
					return false, errors.New("invalid publication file name")
				}
				info, err := os.Stat(filepath.Join(job.OutDir, name))
				if err != nil {
					return false, err
				}
				if !info.Mode().IsRegular() {
					return false, errors.New("published track is not a regular file")
				}
			}
			complete = true
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if complete {
		if err := os.RemoveAll(aside); err != nil {
			return false, err
		}
	} else if _, err := os.Stat(job.OutDir); os.IsNotExist(err) {
		if _, err := os.Stat(aside); err == nil {
			if err := os.Rename(aside, job.OutDir); err != nil {
				return false, err
			}
		} else if !os.IsNotExist(err) {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(publishing); err != nil {
		return false, err
	}
	if err := CleanupStaging(StagingDir(parent, job.ID)); err != nil {
		return false, err
	}
	return complete, nil
}
