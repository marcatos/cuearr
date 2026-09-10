package shntool_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/splitter/native"
	"github.com/marcatos/cuearr/internal/adapters/splitter/shntool"
	"github.com/marcatos/cuearr/internal/domain"
)

type fakeRunner struct {
	exitCode int
	stdout   string
	stderr   string
	err      error
	lastName string
	lastArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	f.lastName = name
	f.lastArgs = append([]string(nil), args...)
	return f.stdout, f.stderr, f.exitCode, f.err
}

func TestShntoolSplit_BuildsExpectedArgs(t *testing.T) {
	fake := &fakeRunner{exitCode: 0, stdout: "ok"}
	s := shntool.New(fake, "shntool")
	plan := domain.SplitPlan{
		CuePath: "/in/album.cue", ImagePath: "/in/album.flac", WorkDir: "/in",
	}
	res, err := s.Split(context.Background(), plan, "/out")
	if err != nil {
		t.Fatal(err)
	}
	if fake.lastName != "shntool" {
		t.Fatalf("bin=%s", fake.lastName)
	}
	joined := strings.Join(fake.lastArgs, " ")
	if !strings.Contains(joined, "split") || !strings.Contains(joined, "/in/album.cue") {
		t.Fatalf("args=%v", fake.lastArgs)
	}
	_ = res
}

func TestNativeStub_NotImplemented(t *testing.T) {
	_, err := native.New().Split(context.Background(), domain.SplitPlan{}, "/out")
	if !errors.Is(err, domain.ErrNotImplemented) {
		t.Fatal(err)
	}
}
