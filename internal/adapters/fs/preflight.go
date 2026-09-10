package fs

import (
	"errors"
	"fmt"
	"os"

	"github.com/marcatos/cuearr/internal/ports"
)

type FileSafetyProbe struct{}

func NewFileSafetyProbe() FileSafetyProbe {
	return FileSafetyProbe{}
}

func (FileSafetyProbe) StatSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%q is not a regular file", path)
	}
	return info.Size(), nil
}

func (FileSafetyProbe) EnsureWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".cuearr-write-check-*")
	if err != nil {
		return err
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	return errors.Join(closeErr, removeErr)
}

func (FileSafetyProbe) FreeSpace(dir string) (uint64, error) {
	return freeSpace(dir)
}

var _ ports.FileSafetyProbe = FileSafetyProbe{}
