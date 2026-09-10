package app

import (
	"path/filepath"
	"strings"

	"github.com/marcatos/cuearr/internal/domain"
)

func DetectAlbum(dir string, readFile func(string) ([]byte, error), listDir func(string) ([]domain.DirEntry, error)) (domain.SplitPlan, error) {
	entries, err := listDir(dir)
	if err != nil {
		return domain.SplitPlan{}, err
	}

	cueName := pickCueFile(entries)
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
	plan.Fingerprint = domain.Fingerprint(plan.CuePath, plan.ImagePath, cueBytes)
	return plan, nil
}

func pickCueFile(entries []domain.DirEntry) string {
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name), ".cue") {
			return e.Name
		}
	}
	return ""
}
