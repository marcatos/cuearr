package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ErrCrossDeviceRename indicates os.Rename cannot atomically move across filesystems.
var ErrCrossDeviceRename = errors.New("cross-device rename not supported")

// PublishMode selects how tracks become visible under finalOut.
type PublishMode int

const (
	// PublishReplaceDir builds a complete album directory then atomically
	// renames it into finalOut. An existing finalOut is moved aside first and
	// only deleted after the new directory is live, so rollback never deletes
	// pre-existing tracks.
	PublishReplaceDir PublishMode = iota
	// PublishMergeInto is retained for callers to receive an explicit error;
	// merging cannot provide atomic album visibility.
	PublishMergeInto
)

var (
	osRename    = os.Rename
	osMkdirAll  = os.MkdirAll
	osRemoveAll = os.RemoveAll
	osStat      = os.Stat
)

// StagingDir returns the per-job staging directory under baseOut.
func StagingDir(baseOut, jobID string) string {
	return filepath.Join(baseOut, ".cuearr-staging", jobID)
}

// PrepareStaging creates a clean staging directory for the job.
func PrepareStaging(baseOut, jobID string) (string, error) {
	staging := StagingDir(baseOut, jobID)
	if err := osRemoveAll(staging); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove existing staging %q: %w", staging, err)
	}
	if err := osMkdirAll(staging, 0o755); err != nil {
		return "", fmt.Errorf("create staging %q: %w", staging, err)
	}
	return staging, nil
}

// CleanupStaging removes the staging directory tree.
func CleanupStaging(staging string) error {
	if staging == "" {
		return nil
	}
	if err := osRemoveAll(staging); err != nil {
		return fmt.Errorf("remove staging %q: %w", staging, err)
	}
	return nil
}

// PublishTracks moves verified tracks from staging into finalOut.
//
// PublishReplaceDir assembles tracks under a sibling temp directory and renames
// that directory into place as a unit. PublishMergeInto is rejected because it
// would expose a partial album.
func PublishTracks(staging, finalOut, jobID string, files []string, mode PublishMode) (published []string, err error) {
	log := slog.Default()
	start := time.Now()
	log.Info("publish tracks start",
		"staging", staging,
		"final_out", finalOut,
		"job_id", jobID,
		"mode", mode.String(),
		"file_count", len(files),
	)
	defer func() {
		log.Info("publish tracks finished",
			"published", len(published),
			"ok", err == nil,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()

	switch mode {
	case PublishMergeInto:
		return nil, errors.New("merge publish is not atomic; use directory replacement")
	default:
		return publishReplaceDir(staging, finalOut, jobID, files)
	}
}

func (m PublishMode) String() string {
	switch m {
	case PublishMergeInto:
		return "merge"
	default:
		return "replace_dir"
	}
}

func publishReplaceDir(staging, finalOut, jobID string, files []string) ([]string, error) {
	parent := filepath.Dir(finalOut)
	base := filepath.Base(finalOut)
	if parent == "" || base == "" || base == "." || base == string(filepath.Separator) {
		return nil, fmt.Errorf("invalid final out %q", finalOut)
	}
	if err := osMkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("ensure final parent %q: %w", parent, err)
	}

	publishDir := filepath.Join(parent, "."+base+".publishing-"+jobID)
	asideDir := filepath.Join(parent, "."+base+".aside-"+jobID)
	if err := osRemoveAll(publishDir); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove leftover publish dir %q: %w", publishDir, err)
	}
	if _, err := osStat(asideDir); err == nil {
		if _, finalErr := osStat(finalOut); os.IsNotExist(finalErr) {
			if restoreErr := renamePath(asideDir, finalOut); restoreErr != nil {
				return nil, fmt.Errorf("restore interrupted album swap: %w", restoreErr)
			}
		} else if finalErr == nil {
			return nil, fmt.Errorf("stale album backup requires recovery: %s", asideDir)
		} else {
			return nil, fmt.Errorf("stat final out %q during recovery: %w", finalOut, finalErr)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat album backup %q: %w", asideDir, err)
	}
	if err := osMkdirAll(publishDir, 0o755); err != nil {
		return nil, fmt.Errorf("create publish dir %q: %w", publishDir, err)
	}

	if err := moveTracks(staging, publishDir, files); err != nil {
		_ = osRemoveAll(publishDir)
		return nil, err
	}

	_, finalErr := osStat(finalOut)
	finalExists := finalErr == nil
	if finalErr != nil && !os.IsNotExist(finalErr) {
		_ = osRemoveAll(publishDir)
		return nil, fmt.Errorf("stat final out %q: %w", finalOut, finalErr)
	}

	if finalExists {
		if err := renamePath(finalOut, asideDir); err != nil {
			_ = osRemoveAll(publishDir)
			return nil, err
		}
		if err := renamePath(publishDir, finalOut); err != nil {
			if restoreErr := renamePath(asideDir, finalOut); restoreErr != nil {
				slog.Default().Error("publish swap restore failed",
					"aside", asideDir, "final_out", finalOut, "error", restoreErr)
			}
			_ = osRemoveAll(publishDir)
			return nil, err
		}
		if err := osRemoveAll(asideDir); err != nil {
			slog.Default().Warn("remove aside album failed", "aside", asideDir, "error", err)
		}
	} else if err := renamePath(publishDir, finalOut); err != nil {
		_ = osRemoveAll(publishDir)
		return nil, err
	}

	return publishedPaths(finalOut, files), nil
}

func moveTracks(staging, destDir string, files []string) error {
	for _, name := range files {
		src := trackSourcePath(staging, name)
		dst := filepath.Join(destDir, filepath.Base(src))
		if err := renamePath(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func publishedPaths(finalOut string, files []string) []string {
	out := make([]string, 0, len(files))
	for _, name := range files {
		out = append(out, filepath.Join(finalOut, filepath.Base(trackSourcePath("", name))))
	}
	return out
}

func renamePath(oldpath, newpath string) error {
	if err := osRename(oldpath, newpath); err != nil {
		if isCrossDeviceRenameErr(err) {
			return fmt.Errorf("%w: %v", ErrCrossDeviceRename, err)
		}
		return fmt.Errorf("rename %q -> %q: %w", oldpath, newpath, err)
	}
	return nil
}

func trackSourcePath(staging, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	if staging == "" {
		return name
	}
	return filepath.Join(staging, name)
}

func isCrossDeviceRenameErr(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}
