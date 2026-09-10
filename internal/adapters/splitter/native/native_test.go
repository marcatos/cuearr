package native_test

import (
	"context"
	"errors"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/splitter/native"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestNativeStub_NotImplemented(t *testing.T) {
	_, err := native.New().Split(context.Background(), domain.SplitPlan{}, "/out")
	if !errors.Is(err, domain.ErrNotImplemented) {
		t.Fatal(err)
	}
}
