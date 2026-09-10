package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

const durationTolerance = 100 * time.Millisecond

var (
	ErrOutputCountMismatch = errors.New("split output count does not match CUE tracks")
	ErrDurationMismatch    = errors.New("split output duration does not match CUE timing")
)

type JobRunner struct {
	Store     ports.JobStore
	Splitter  ports.Splitter
	Inspector ports.FLACInspector
	Tagger    ports.FLACTagger
	Preflight ports.JobPreflight
	ReadFile  func(string) ([]byte, error)
	Log       *slog.Logger
}

func (r JobRunner) VerifySplit(
	ctx context.Context,
	cuePath string,
	outputFiles []string,
	totalImageDuration time.Duration,
) (err error) {
	start := time.Now()
	log := r.logger()
	log.Info("split verification start", "cue_path", cuePath, "output_files", len(outputFiles))
	defer func() {
		attrs := []any{
			"cue_path", cuePath,
			"output_files", len(outputFiles),
			"duration_ms", time.Since(start).Milliseconds(),
			"ok", err == nil,
		}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
			log.Error("split verification failed", attrs...)
			return
		}
		log.Info("split verification finished", attrs...)
	}()

	readFile := r.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	cueBytes, err := readFile(cuePath)
	if err != nil {
		return fmt.Errorf("read CUE for verification: %w", err)
	}
	sheet, err := domain.ParseCue(cueBytes)
	if err != nil {
		return fmt.Errorf("parse CUE for verification: %w", err)
	}
	if len(outputFiles) != len(sheet.Tracks) {
		return fmt.Errorf("%w: got %d outputs for %d tracks",
			ErrOutputCountMismatch, len(outputFiles), len(sheet.Tracks))
	}

	expected, err := domain.ExpectedTrackDurations(sheet, totalImageDuration)
	if err != nil {
		return fmt.Errorf("derive expected track durations: %w", err)
	}
	for i, path := range outputFiles {
		info, inspectErr := r.Inspector.Inspect(ctx, path)
		if inspectErr != nil {
			return fmt.Errorf("inspect output track %d %q: %w", sheet.Tracks[i].Number, path, inspectErr)
		}
		delta := info.Duration - expected[i]
		if delta < 0 {
			delta = -delta
		}
		if delta > durationTolerance {
			return fmt.Errorf("%w: track %d %q got %v, expected %v (delta %v)",
				ErrDurationMismatch, sheet.Tracks[i].Number, path, info.Duration, expected[i], delta)
		}
	}
	log.Info("split durations verified", "cue_path", cuePath, "tracks", len(outputFiles))

	for i, path := range outputFiles {
		track := sheet.Tracks[i]
		artist := track.Performer
		if artist == "" {
			artist = sheet.Performer
		}
		tags := ports.TrackTags{
			Title:       track.Title,
			Artist:      artist,
			Album:       sheet.Title,
			TrackNumber: strconv.Itoa(track.Number),
		}
		if err := r.Tagger.ApplyTags(ctx, path, tags); err != nil {
			return fmt.Errorf("tag output track %d %q: %w", track.Number, path, err)
		}
	}
	log.Info("split tags applied", "cue_path", cuePath, "tracks", len(outputFiles))
	return nil
}

func (r JobRunner) logger() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}
