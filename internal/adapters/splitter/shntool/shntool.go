package shntool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type Splitter struct {
	runner  ports.CommandRunner
	binPath string
}

func New(runner ports.CommandRunner, binPath string) *Splitter {
	return &Splitter{runner: runner, binPath: binPath}
}

func (s *Splitter) Name() string {
	return "shntool"
}

func (s *Splitter) Available(ctx context.Context) error {
	_, stderr, exitCode, err := s.runner.Run(ctx, s.binPath, "version")
	if err != nil {
		return fmt.Errorf("shntool available: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("shntool available: exit %d: %s", exitCode, stderr)
	}
	return nil
}

func (s *Splitter) Split(ctx context.Context, plan domain.SplitPlan, outDir string) (ports.SplitResult, error) {
	start := time.Now()
	args := []string{
		"split",
		"-f", plan.CuePath,
		"-o", "flac",
		"-d", outDir,
		plan.ImagePath,
	}
	stdout, stderr, exitCode, err := s.runner.Run(ctx, s.binPath, args...)
	logParts := []string{stdout, stderr}
	log := strings.TrimSpace(strings.Join(logParts, "\n"))
	duration := time.Since(start)

	if err != nil {
		return ports.SplitResult{Log: log, Duration: duration}, err
	}
	if exitCode != 0 {
		return ports.SplitResult{Log: log, Duration: duration},
			fmt.Errorf("%w: %s", domain.ErrSplitFailed, strings.TrimSpace(stderr))
	}

	return ports.SplitResult{
		Log:      log,
		Duration: duration,
	}, nil
}
