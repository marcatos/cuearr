package ports

import "context"

// FileSafetyProbe exposes filesystem operations needed by the preflight use case.
type FileSafetyProbe interface {
	StatSize(path string) (int64, error)
	EnsureWritable(dir string) error
	FreeSpace(dir string) (uint64, error)
}

// JobPreflight validates that a split can start without risking a partial copy.
type JobPreflight interface {
	Check(ctx context.Context, imagePath, outputParent string) error
}
