package wavduration_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/audio/wavduration"
)

func TestInspect_PCMDuration(t *testing.T) {
	const (
		sampleRate = 44100
		channels   = 2
		bits       = 16
		seconds    = 6
	)
	path := writeWAV(t, pcmWAV(sampleRate, channels, bits, seconds))

	info, err := wavduration.New(nil).Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if info.SampleRate != sampleRate {
		t.Fatalf("SampleRate=%d, want %d", info.SampleRate, sampleRate)
	}
	if info.TotalSamples != sampleRate*seconds {
		t.Fatalf("TotalSamples=%d, want %d", info.TotalSamples, sampleRate*seconds)
	}
	if info.Duration != seconds*time.Second {
		t.Fatalf("Duration=%v, want %v", info.Duration, seconds*time.Second)
	}
}

func TestInspect_RejectsNonPCMEncoding(t *testing.T) {
	data := pcmWAV(44100, 2, 16, 1)
	binary.LittleEndian.PutUint16(data[20:22], 3)
	path := writeWAV(t, data)

	_, err := wavduration.New(nil).Inspect(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "unsupported WAV encoding") {
		t.Fatalf("err=%v, want unsupported WAV encoding", err)
	}
}

func writeWAV(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "album.wav")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func pcmWAV(sampleRate, channels, bits, seconds int) []byte {
	blockAlign := channels * bits / 8
	dataSize := sampleRate * blockAlign * seconds
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+dataSize))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&b, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(sampleRate*blockAlign))
	_ = binary.Write(&b, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(&b, binary.LittleEndian, uint16(bits))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(dataSize))
	b.Write(make([]byte, dataSize))
	return b.Bytes()
}
