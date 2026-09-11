package ports

import "context"

type LidarrClient interface {
	Ping(ctx context.Context) error
	RequestImport(ctx context.Context, albumPath string) error
}
