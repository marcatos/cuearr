package fs

import "errors"

var (
	errOnDirRequired = errors.New("onDir callback is required")
	errNoWatchDirs   = errors.New("at least one watch directory is required")
)
