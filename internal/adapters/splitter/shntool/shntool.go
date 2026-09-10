package shntool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	stdout, stderr, _, err := s.runner.Run(ctx, s.binPath, "-h")
	if err != nil {
		return fmt.Errorf("shntool available: %w", err)
	}
	combined := stdout + stderr
	if !strings.Contains(strings.ToLower(combined), "shntool") {
		return fmt.Errorf("shntool available: help output did not mention shntool")
	}
	return nil
}

func (s *Splitter) Split(ctx context.Context, plan domain.SplitPlan, outDir string) (ports.SplitResult, error) {
	start := time.Now()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return ports.SplitResult{}, fmt.Errorf("create output directory %q: %w", outDir, err)
	}
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

	outputFiles, err := listOutputFlacFiles(outDir, plan.ImagePath)
	if err != nil {
		return ports.SplitResult{Log: log, Duration: duration}, err
	}
	if len(outputFiles) == 0 {
		return ports.SplitResult{Log: log, Duration: duration},
			fmt.Errorf("%w: no FLAC output files in %s", domain.ErrSplitFailed, outDir)
	}

	return ports.SplitResult{
		OutputFiles: outputFiles,
		Log:         log,
		Duration:    duration,
	}, nil
}

func listOutputFlacFiles(outDir, sourceImage string) ([]string, error) {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, fmt.Errorf("list output FLAC files in %q: %w", outDir, err)
	}
	sourceAbs, err := filepath.Abs(sourceImage)
	if err != nil {
		return nil, fmt.Errorf("absolute path for source image %q: %w", sourceImage, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".flac") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	files := make([]string, 0, len(names))
	for _, name := range names {
		abs, err := filepath.Abs(filepath.Join(outDir, name))
		if err != nil {
			return nil, fmt.Errorf("absolute path for output FLAC %q: %w", name, err)
		}
		if abs == sourceAbs {
			continue
		}
		files = append(files, abs)
	}
	return files, nil
}
