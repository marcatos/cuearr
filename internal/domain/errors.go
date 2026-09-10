package domain

import "errors"

var (
	ErrImageNotFound  = errors.New("image file not found for CUE sheet")
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
	ErrSplitFailed    = errors.New("split failed")
)
