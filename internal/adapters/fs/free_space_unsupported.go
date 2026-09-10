//go:build !linux && !windows

package fs

import (
	"errors"
	"runtime"
)

func freeSpace(string) (uint64, error) {
	return 0, errors.New("free-space query is unsupported on " + runtime.GOOS)
}
