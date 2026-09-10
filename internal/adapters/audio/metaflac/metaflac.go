package metaflac

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/ports"
)

var tagFields = []string{"TITLE", "ARTIST", "ALBUM", "TRACKNUMBER"}

type Client struct {
	runner  ports.CommandRunner
	binPath string
}

func New(runner ports.CommandRunner, binPath string) *Client {
	return &Client{runner: runner, binPath: binPath}
}

func (c *Client) Inspect(ctx context.Context, path string) (ports.FLACInfo, error) {
	rateOut, rateErr, rateCode, err := c.runner.Run(ctx, c.binPath, "--show-sample-rate", path)
	if err != nil {
		return ports.FLACInfo{}, fmt.Errorf("metaflac sample rate: %w", err)
	}
	if rateCode != 0 {
		return ports.FLACInfo{}, fmt.Errorf("metaflac sample rate: exit %d: %s", rateCode, strings.TrimSpace(rateErr))
	}
	sampleRate, err := strconv.Atoi(strings.TrimSpace(rateOut))
	if err != nil {
		return ports.FLACInfo{}, fmt.Errorf("metaflac sample rate: parse %q: %w", strings.TrimSpace(rateOut), err)
	}
	if sampleRate <= 0 {
		return ports.FLACInfo{}, fmt.Errorf("metaflac sample rate: invalid rate %d", sampleRate)
	}

	samplesOut, samplesErr, samplesCode, err := c.runner.Run(ctx, c.binPath, "--show-total-samples", path)
	if err != nil {
		return ports.FLACInfo{}, fmt.Errorf("metaflac total samples: %w", err)
	}
	if samplesCode != 0 {
		return ports.FLACInfo{}, fmt.Errorf("metaflac total samples: exit %d: %s", samplesCode, strings.TrimSpace(samplesErr))
	}
	totalSamples, err := strconv.ParseInt(strings.TrimSpace(samplesOut), 10, 64)
	if err != nil {
		return ports.FLACInfo{}, fmt.Errorf("metaflac total samples: parse %q: %w", strings.TrimSpace(samplesOut), err)
	}

	duration := time.Duration(totalSamples) * time.Second / time.Duration(sampleRate)
	return ports.FLACInfo{
		SampleRate:   sampleRate,
		TotalSamples: totalSamples,
		Duration:     duration,
	}, nil
}

func (c *Client) ApplyTags(ctx context.Context, path string, tags ports.TrackTags) error {
	args := make([]string, 0, len(tagFields)*2+5)
	for _, field := range tagFields {
		args = append(args, "--remove-tag="+field)
	}
	args = append(args,
		"--set-tag=TITLE="+tags.Title,
		"--set-tag=ARTIST="+tags.Artist,
		"--set-tag=ALBUM="+tags.Album,
		"--set-tag=TRACKNUMBER="+tags.TrackNumber,
		path,
	)
	_, stderr, exitCode, err := c.runner.Run(ctx, c.binPath, args...)
	if err != nil {
		return fmt.Errorf("metaflac apply tags: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("metaflac apply tags: exit %d: %s", exitCode, strings.TrimSpace(stderr))
	}
	return nil
}
