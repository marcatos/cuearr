package wavduration

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/marcatos/cuearr/internal/ports"
)

const (
	pcmEncoding = 1
	chunkHeader = 8
)

type Inspector struct {
	log *slog.Logger
}

func New(log *slog.Logger) *Inspector {
	if log == nil {
		log = slog.Default()
	}
	return &Inspector{log: log}
}

func (i *Inspector) Inspect(ctx context.Context, path string) (info ports.FLACInfo, err error) {
	start := time.Now()
	i.log.Info("WAV inspection started", "path", path)
	defer func() {
		i.log.Info("WAV inspection finished",
			"path", path,
			"duration_ms", time.Since(start).Milliseconds(),
			"ok", err == nil,
		)
	}()

	if err := ctx.Err(); err != nil {
		return ports.FLACInfo{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return ports.FLACInfo{}, fmt.Errorf("open WAV: %w", err)
	}
	defer file.Close()

	return inspect(file)
}

func inspect(reader io.ReadSeeker) (ports.FLACInfo, error) {
	var header [12]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return ports.FLACInfo{}, fmt.Errorf("read WAV header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return ports.FLACInfo{}, errors.New("invalid WAV RIFF header")
	}

	var sampleRate, blockAlign int64
	var dataSize int64 = -1
	var formatFound bool
	for {
		var chunk [chunkHeader]byte
		if _, err := io.ReadFull(reader, chunk[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return ports.FLACInfo{}, fmt.Errorf("read WAV chunk: %w", err)
		}
		size := int64(binary.LittleEndian.Uint32(chunk[4:8]))
		switch string(chunk[0:4]) {
		case "fmt ":
			if size < 16 {
				return ports.FLACInfo{}, errors.New("invalid WAV fmt chunk")
			}
			var format [16]byte
			if _, err := io.ReadFull(reader, format[:]); err != nil {
				return ports.FLACInfo{}, fmt.Errorf("read WAV format: %w", err)
			}
			encoding := binary.LittleEndian.Uint16(format[0:2])
			if encoding != pcmEncoding {
				return ports.FLACInfo{}, fmt.Errorf("unsupported WAV encoding %d: only PCM is supported", encoding)
			}
			sampleRate = int64(binary.LittleEndian.Uint32(format[4:8]))
			blockAlign = int64(binary.LittleEndian.Uint16(format[12:14]))
			if sampleRate <= 0 || blockAlign <= 0 {
				return ports.FLACInfo{}, errors.New("invalid WAV sample rate or block alignment")
			}
			formatFound = true
			if err := skip(reader, size-16); err != nil {
				return ports.FLACInfo{}, err
			}
		case "data":
			dataSize = size
			if formatFound {
				return wavInfo(sampleRate, blockAlign, dataSize)
			}
			if err := skip(reader, size); err != nil {
				return ports.FLACInfo{}, err
			}
		default:
			if err := skip(reader, size); err != nil {
				return ports.FLACInfo{}, err
			}
		}
		if size%2 != 0 {
			if err := skip(reader, 1); err != nil {
				return ports.FLACInfo{}, err
			}
		}
	}

	if !formatFound {
		return ports.FLACInfo{}, errors.New("WAV fmt chunk not found")
	}
	if dataSize < 0 {
		return ports.FLACInfo{}, errors.New("WAV data chunk not found")
	}
	return wavInfo(sampleRate, blockAlign, dataSize)
}

func skip(reader io.Seeker, size int64) error {
	if _, err := reader.Seek(size, io.SeekCurrent); err != nil {
		return fmt.Errorf("skip WAV chunk: %w", err)
	}
	return nil
}

func wavInfo(sampleRate, blockAlign, dataSize int64) (ports.FLACInfo, error) {
	if dataSize%blockAlign != 0 {
		return ports.FLACInfo{}, errors.New("WAV data size is not sample-aligned")
	}
	totalSamples := dataSize / blockAlign
	return ports.FLACInfo{
		SampleRate:   int(sampleRate),
		TotalSamples: totalSamples,
		Duration:     time.Duration(totalSamples) * time.Second / time.Duration(sampleRate),
	}, nil
}
