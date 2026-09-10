package metaflac_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/audio/metaflac"
	"github.com/marcatos/cuearr/internal/ports"
)

type seqRunner struct {
	responses []runResult
	calls     [][]string
}

type runResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (s *seqRunner) Run(_ context.Context, _ string, args ...string) (stdout, stderr string, exitCode int, err error) {
	s.calls = append(s.calls, append([]string(nil), args...))
	if len(s.responses) == 0 {
		return "", "unexpected run", 1, nil
	}
	r := s.responses[0]
	s.responses = s.responses[1:]
	return r.stdout, r.stderr, r.exitCode, r.err
}

func TestInspect_ParsesSampleRateAndTotalSamples(t *testing.T) {
	runner := &seqRunner{responses: []runResult{
		{stdout: "44100\n", exitCode: 0},
		{stdout: "88200\n", exitCode: 0},
	}}
	c := metaflac.New(runner, "metaflac")

	info, err := c.Inspect(context.Background(), "/music/track.flac")
	if err != nil {
		t.Fatal(err)
	}
	if info.SampleRate != 44100 {
		t.Fatalf("SampleRate=%d want 44100", info.SampleRate)
	}
	if info.TotalSamples != 88200 {
		t.Fatalf("TotalSamples=%d want 88200", info.TotalSamples)
	}
	wantDur := 2 * time.Second
	if info.Duration != wantDur {
		t.Fatalf("Duration=%v want %v", info.Duration, wantDur)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls=%d want 2", len(runner.calls))
	}
	if !reflect.DeepEqual(runner.calls[0], []string{"--show-sample-rate", "/music/track.flac"}) {
		t.Fatalf("call0=%v", runner.calls[0])
	}
	if !reflect.DeepEqual(runner.calls[1], []string{"--show-total-samples", "/music/track.flac"}) {
		t.Fatalf("call1=%v", runner.calls[1])
	}
}

func TestInspect_NonzeroExitReturnsError(t *testing.T) {
	const stderrMsg = "ERROR: not a FLAC file"
	runner := &seqRunner{responses: []runResult{
		{stderr: stderrMsg, exitCode: 1},
	}}
	c := metaflac.New(runner, "metaflac")

	_, err := c.Inspect(context.Background(), "/bad.flac")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), stderrMsg) {
		t.Fatalf("stderr not in error: %v", err)
	}
}

func TestInspect_RunnerErrorPropagates(t *testing.T) {
	runErr := errors.New("exec failed")
	runner := &seqRunner{responses: []runResult{{err: runErr}}}
	c := metaflac.New(runner, "metaflac")

	_, err := c.Inspect(context.Background(), "/music/track.flac")
	if !errors.Is(err, runErr) {
		t.Fatalf("got %v", err)
	}
}

func TestApplyTags_BuildsRemoveAndSetArgs(t *testing.T) {
	runner := &seqRunner{responses: []runResult{{exitCode: 0}}}
	c := metaflac.New(runner, "metaflac")
	tags := ports.TrackTags{
		Title:       "Track One",
		Artist:      "Artist",
		Album:       "Album",
		TrackNumber: "1",
	}

	if err := c.ApplyTags(context.Background(), "/out/01.flac", tags); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls=%d want 1", len(runner.calls))
	}
	want := []string{
		"--remove-tag=TITLE",
		"--remove-tag=ARTIST",
		"--remove-tag=ALBUM",
		"--remove-tag=TRACKNUMBER",
		"--set-tag=TITLE=Track One",
		"--set-tag=ARTIST=Artist",
		"--set-tag=ALBUM=Album",
		"--set-tag=TRACKNUMBER=1",
		"/out/01.flac",
	}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("args=%v want=%v", runner.calls[0], want)
	}
}

func TestApplyTags_NonzeroExitReturnsError(t *testing.T) {
	runner := &seqRunner{responses: []runResult{{stderr: "tag failed", exitCode: 2}}}
	c := metaflac.New(runner, "metaflac")

	err := c.ApplyTags(context.Background(), "/out/01.flac", ports.TrackTags{Title: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
