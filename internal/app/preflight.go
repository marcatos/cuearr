package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/marcatos/cuearr/internal/ports"
)

const (
	defaultStabilityGap = 500 * time.Millisecond
	freeSpaceReserve    = uint64(64 * 1024 * 1024)
)

var (
	ErrImageUnstable        = errors.New("source image is still changing")
	ErrOutputNotWritable    = errors.New("output location is not writable")
	ErrInsufficientSpace    = errors.New("insufficient free space")
	ErrFreeSpaceUnavailable = errors.New("free-space check unavailable")
	ErrPreflightUnavailable = errors.New("job preflight is not configured")
)

// Preflight checks source stability and destination safety before splitting.
type Preflight struct {
	Probe        ports.FileSafetyProbe
	StabilityGap time.Duration
	Wait         func(context.Context, time.Duration) error
	Log          *slog.Logger
}

func (p Preflight) Check(ctx context.Context, imagePath, outputParent string) (err error) {
	start := time.Now()
	log := p.logger()
	log.Info("job preflight start", "image_path", imagePath, "output_parent", outputParent)
	defer func() {
		log.Info("job preflight finished",
			"image_path", imagePath,
			"output_parent", outputParent,
			"duration_ms", time.Since(start).Milliseconds(),
			"ok", err == nil,
		)
	}()

	if p.Probe == nil {
		return ErrPreflightUnavailable
	}
	firstSize, err := p.Probe.StatSize(imagePath)
	if err != nil {
		return fmt.Errorf("stat source image %q: %w", imagePath, err)
	}
	if firstSize < 0 {
		return fmt.Errorf("stat source image %q: invalid size %d", imagePath, firstSize)
	}
	if err := p.Probe.EnsureWritable(outputParent); err != nil {
		return fmt.Errorf("%w: %q: %v", ErrOutputNotWritable, outputParent, err)
	}

	available, err := p.Probe.FreeSpace(outputParent)
	if err != nil {
		return fmt.Errorf("%w for %q: %v", ErrFreeSpaceUnavailable, outputParent, err)
	}
	required := requiredFreeSpace(uint64(firstSize))
	if available < required {
		return fmt.Errorf("%w: available=%d required=%d image_size=%d",
			ErrInsufficientSpace, available, required, firstSize)
	}
	log.Info("job preflight destination checked",
		"output_parent", outputParent,
		"available_bytes", available,
		"required_bytes", required,
	)

	if err := p.wait(ctx, p.stabilityGap()); err != nil {
		return fmt.Errorf("wait for source stability: %w", err)
	}
	secondSize, err := p.Probe.StatSize(imagePath)
	if err != nil {
		return fmt.Errorf("restat source image %q: %w", imagePath, err)
	}
	if secondSize != firstSize {
		return fmt.Errorf("%w: %q changed from %d to %d bytes",
			ErrImageUnstable, imagePath, firstSize, secondSize)
	}
	log.Info("job preflight source stable", "image_path", imagePath, "size_bytes", firstSize)
	return nil
}

func (p Preflight) stabilityGap() time.Duration {
	if p.StabilityGap >= defaultStabilityGap {
		return p.StabilityGap
	}
	return defaultStabilityGap
}

func (p Preflight) wait(ctx context.Context, duration time.Duration) error {
	if p.Wait != nil {
		return p.Wait(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p Preflight) logger() *slog.Logger {
	if p.Log != nil {
		return p.Log
	}
	return slog.Default()
}

func requiredFreeSpace(imageSize uint64) uint64 {
	double := imageSize * 2
	if imageSize > math.MaxUint64/2 {
		double = math.MaxUint64
	}
	withReserve := imageSize + freeSpaceReserve
	if imageSize > math.MaxUint64-freeSpaceReserve {
		withReserve = math.MaxUint64
	}
	return max(double, withReserve)
}

var _ ports.JobPreflight = Preflight{}
