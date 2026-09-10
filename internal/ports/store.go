package ports

import (
	"context"

	"github.com/marcatos/cuearr/internal/domain"
)

type JobStore interface {
	Create(ctx context.Context, job domain.Job) (domain.Job, error)
	Get(ctx context.Context, id string) (domain.Job, error)
	List(ctx context.Context, limit int) ([]domain.Job, error)
	Update(ctx context.Context, job domain.Job) error
	FindByFingerprint(ctx context.Context, fp string) (domain.Job, error)
}

type RunningJobRecoverer interface {
	RecoverRunning(ctx context.Context) (int64, error)
}

type SettingsStore interface {
	Get(ctx context.Context) (domain.Settings, error)
	Put(ctx context.Context, s domain.Settings) error
}
