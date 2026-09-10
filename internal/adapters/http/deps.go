package httpapi

import (
	"context"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type Deps struct {
	Jobs         ports.JobStore
	Settings     ports.SettingsStore
	Engine       string
	WatchDirs    []string
	CreateJob    func(ctx context.Context, path string) (domain.Job, bool, error)
	ScanWatch    func(ctx context.Context) error
	CheckSQLite  func(ctx context.Context) error
	CheckShntool func(ctx context.Context) error
}
