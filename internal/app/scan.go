package app

import (
	"context"

	fsadapter "github.com/marcatos/cuearr/internal/adapters/fs"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

func ScanAll(watchDirs []string, maxDepth int, onDir func(dir string) error) error {
	dirs, err := fsadapter.WalkCueDirs(watchDirs, maxDepth)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if err := onDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func ScanDir(
	ctx context.Context,
	dir string,
	store ports.JobStore,
	engine string,
	readFile func(string) ([]byte, error),
	listDir func(string) ([]domain.DirEntry, error),
) (domain.Job, bool, error) {
	plan, err := DetectAlbum(dir, readFile, listDir, nil, nil)
	if err != nil {
		return domain.Job{}, false, err
	}
	return Enqueue(ctx, store, plan, engine)
}
