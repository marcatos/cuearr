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

func (w *Worker) recoverPublications(ctx context.Context) error {
	lister, ok := w.Store.(ports.PublishingJobLister)
	if !ok {
		return nil
	}
	jobs, err := lister.ListPublishing(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		complete, err := recoverPublication(job)
		if err != nil {
			return fmt.Errorf("recover publication %s: %w", job.ID, err)
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
