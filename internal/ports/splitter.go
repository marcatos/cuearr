package ports

import (
	"context"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

type Splitter interface {
	Name() string
	Available(ctx context.Context) error
	Split(ctx context.Context, plan domain.SplitPlan, outDir string) (result SplitResult, err error)
}

type SplitResult struct {
	OutputFiles []string
	Log         string
	Duration    time.Duration
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error)
}
