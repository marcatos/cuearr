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

var (
	osRename    = os.Rename
	osMkdirAll  = os.MkdirAll
	osRemoveAll = os.RemoveAll
	osRemove    = os.Remove
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

// PublishTracks atomically moves track files from staging into finalOut via os.Rename.
// On failure, best-effort removes files already moved into finalOut during this call.
func PublishTracks(staging string, finalOut string, files []string) (published []string, err error) {
	log := slog.Default()
	start := time.Now()
	log.Info("publish tracks start",
		"staging", staging,
		"final_out", finalOut,
		"file_count", len(files),
	)
	defer func() {
		log.Info("publish tracks finished",
			"published", len(published),
			"ok", err == nil,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()

	if err := osMkdirAll(finalOut, 0o755); err != nil {
		return nil, fmt.Errorf("ensure final out %q: %w", finalOut, err)
	}

	published = make([]string, 0, len(files))
	for _, name := range files {
		src := trackSourcePath(staging, name)
		dst := filepath.Join(finalOut, filepath.Base(src))

		if renameErr := osRename(src, dst); renameErr != nil {
			if isCrossDeviceRenameErr(renameErr) {
				renameErr = fmt.Errorf("%w: %v", ErrCrossDeviceRename, renameErr)
			} else {
				renameErr = fmt.Errorf("rename %q -> %q: %w", src, dst, renameErr)
			}
			rollbackPartialPublish(published)
			return nil, renameErr
		}
		published = append(published, dst)
	}
	return published, nil
}

func trackSourcePath(staging, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(staging, name)
}

func rollbackPartialPublish(published []string) {
	for _, path := range published {
		if err := osRemove(path); err != nil && !os.IsNotExist(err) {
			slog.Default().Warn("rollback partial publish failed", "path", path, "error", err)
		}
	}
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
