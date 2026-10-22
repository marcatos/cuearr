package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

func DetectAlbum(
	ctx context.Context,
	dir string,
	readFile func(string) ([]byte, error),
	listDir func(string) ([]domain.DirEntry, error),
	statFile func(string) (domain.FileStat, error),
	hashFile func(string) (string, error),
) (domain.SplitPlan, error) {
	if err := ctx.Err(); err != nil {
		return domain.SplitPlan{}, err
	}
	if statFile == nil {
		statFile = statImageFile
	}
	if hashFile == nil {
		hashFile = func(path string) (string, error) { return domain.HashFileContent(ctx, path) }
	}

	entries, err := listDir(dir)
	if err != nil {
		return domain.SplitPlan{}, err
	}

	cueName, err := domain.PickSingleCueFile(entries)
	if err != nil {
		return domain.SplitPlan{}, err
	}
	if cueName == "" {
		return domain.SplitPlan{}, domain.ErrImageNotFound
	}

	cuePath := filepath.Join(dir, cueName)
	cueBytes, err := readFile(cuePath)
	if err != nil {
		return domain.SplitPlan{}, err
	}

	sheet, err := domain.ParseCue(cueBytes)
	if err != nil {
		return domain.SplitPlan{}, err
	}

	plan, err := domain.BuildSplitPlan(dir, sheet, entries)
	if err != nil {
		return domain.SplitPlan{}, err
	}

	img, err := imageIdentity(plan.ImagePath, statFile, hashFile)
	if err != nil {
		return domain.SplitPlan{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.SplitPlan{}, err
	}
	plan.Fingerprint = domain.Fingerprint(plan.CuePath, plan.ImagePath, cueBytes, img)
	return plan, nil
}

func statImageFile(path string) (domain.FileStat, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return domain.FileStat{}, err
	}
	return domain.FileStat{
		Size:            fi.Size(),
		ModTimeUnixNano: fi.ModTime().UnixNano(),
	}, nil
}

func imageIdentity(imagePath string, statFile func(string) (domain.FileStat, error), hashFile func(string) (string, error)) (domain.ImageIdentity, error) {
	st, err := statFile(imagePath)
	if err != nil {
		return domain.ImageIdentity{}, err
	}
	start := time.Now()
	contentHash, err := hashFile(imagePath)
	if err != nil {
		return domain.ImageIdentity{}, err
	}
	slog.Default().Info("image hashed for fingerprint",
		"path", imagePath,
		"bytes", st.Size,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return domain.ImageIdentity{
		Size:            st.Size,
		ModTimeUnixNano: st.ModTimeUnixNano,
		ContentSHA256:   contentHash,
	}, nil
}
