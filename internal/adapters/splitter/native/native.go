package native

import (
	"context"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type Splitter struct{}

func New() *Splitter {
	return &Splitter{}
}

func (s *Splitter) Name() string {
	return "native"
}

func (s *Splitter) Available(_ context.Context) error {
	return domain.ErrNotImplemented
}

func (s *Splitter) Split(_ context.Context, _ domain.SplitPlan, _ string) (ports.SplitResult, error) {
	return ports.SplitResult{}, domain.ErrNotImplemented
}
