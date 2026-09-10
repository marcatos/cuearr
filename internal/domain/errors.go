package domain

import "errors"

var (
	ErrImageNotFound  = errors.New("image file not found for CUE sheet")
	ErrAmbiguousCue   = errors.New("multiple CUE sheets found")
	ErrMultiFileCue   = errors.New("CUE sheet contains multiple FILE directives")
	ErrConflict       = errors.New("conflict")
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
	ErrSplitFailed    = errors.New("split failed")
)
