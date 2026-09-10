package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

func Enqueue(ctx context.Context, store ports.JobStore, plan domain.SplitPlan, engine string) (domain.Job, bool, error) {
	existing, err := store.FindByFingerprint(ctx, plan.Fingerprint)
	if err == nil {
		switch existing.Status {
		case domain.JobCompleted, domain.JobQueued, domain.JobRunning:
			return existing, false, nil
		case domain.JobFailed:
			retry := existing
			retry.Status = domain.JobQueued
			retry.Error = ""
			retry.Log = ""
			retry.StartedAt = time.Time{}
			retry.FinishedAt = time.Time{}
			if err := store.Update(ctx, retry); err != nil {
				return domain.Job{}, false, err
			}
			return retry, true, nil
		default:
			return existing, false, nil
		}
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Job{}, false, err
	}

	now := time.Now().UTC()
	job := domain.Job{
		ID:          uuid.NewString(),
		Fingerprint: plan.Fingerprint,
		CuePath:     plan.CuePath,
		ImagePath:   plan.ImagePath,
		Status:      domain.JobQueued,
		Engine:      engine,
		CreatedAt:   now,
	}
	created, err := store.Create(ctx, job)
	if err != nil {
		return domain.Job{}, false, err
	}
	return created, true, nil
}
