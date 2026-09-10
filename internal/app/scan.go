package app

import (
	"context"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

func ScanDir(
	ctx context.Context,
	dir string,
	store ports.JobStore,
	engine string,
	readFile func(string) ([]byte, error),
	listDir func(string) ([]domain.DirEntry, error),
) (domain.Job, bool, error) {
	plan, err := DetectAlbum(dir, readFile, listDir)
	if err != nil {
		return domain.Job{}, false, err
	}
	return Enqueue(ctx, store, plan, engine)
}
