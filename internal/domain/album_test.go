package domain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cancelAfterRead struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (r cancelAfterRead) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.cancel()
	return n, err
}

func TestHashReaderStopsBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := bytes.NewReader(make([]byte, 1<<20))
	_, err := io.Copy(io.Discard, contextReader{ctx: ctx, reader: cancelAfterRead{reader: source, cancel: cancel}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if source.Len() == 0 {
		t.Fatal("reader consumed entire image after cancellation")
	}
}

func testImageIdentity(size int64, mtime int64, contentHex string) ImageIdentity {
	return ImageIdentity{
		Size:            size,
		ModTimeUnixNano: mtime,
		ContentSHA256:   contentHex,
	}
}

func expectedFingerprint(cuePath, imagePath string, cueBytes []byte, img ImageIdentity) string {
	cueHash := sha256.Sum256(cueBytes)
	cueHex := hex.EncodeToString(cueHash[:])
	payload := fmt.Sprintf("%s|%s|%s|%d|%d|%s",
		cuePath, imagePath, cueHex, img.Size, img.ModTimeUnixNano, img.ContentSHA256)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func TestFingerprint_Deterministic(t *testing.T) {
	cueBytes := []byte("FILE \"a.flac\" WAVE\n")
	img := testImageIdentity(42, 1700000000000000000, "deadbeef")
	got := Fingerprint("/album/x.cue", "/album/a.flac", cueBytes, img)
	want := expectedFingerprint("/album/x.cue", "/album/a.flac", cueBytes, img)
	if got != want {
		t.Fatalf("fingerprint=%q want=%q", got, want)
	}
}

func TestFingerprint_DifferentImageContentDifferentFingerprint(t *testing.T) {
	cuePath := "/album/x.cue"
	imagePath := "/album/a.flac"
	cueBytes := []byte("FILE \"a.flac\" WAVE\n")
	base := testImageIdentity(100, 1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	other := testImageIdentity(100, 1, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if Fingerprint(cuePath, imagePath, cueBytes, base) == Fingerprint(cuePath, imagePath, cueBytes, other) {
		t.Fatal("expected different fingerprints when image content hash differs")
	}
}

func TestFingerprint_IdenticalInputsSameFingerprint(t *testing.T) {
	cuePath := "/album/x.cue"
	imagePath := "/album/a.flac"
	cueBytes := []byte("FILE \"a.flac\" WAVE\n")
	img := testImageIdentity(999, 42, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	a := Fingerprint(cuePath, imagePath, cueBytes, img)
	b := Fingerprint(cuePath, imagePath, cueBytes, img)
	if a != b {
		t.Fatalf("a=%q b=%q", a, b)
	}
}

func TestHashFileContent_SHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.flac")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := HashFileContent(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("hash=%q want=%q", got, want)
	}
}

func TestBuildSplitPlan_AutoDetectSingleFlac(t *testing.T) {
	sheet := CueSheet{Tracks: []CueTrack{{Number: 1, Index01: "00:00:00"}}}
	entries := []DirEntry{{Name: "only.flac"}, {Name: "album.cue"}}
	plan, err := BuildSplitPlan("/music/album", sheet, entries)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ImagePath != filepath.Join("/music/album", "only.flac") {
		t.Fatalf("image=%q", plan.ImagePath)
	}
}

func TestBuildSplitPlan_FindsCuePath(t *testing.T) {
	sheet := CueSheet{File: "album.flac", Tracks: []CueTrack{{Number: 1}}}
	entries := []DirEntry{{Name: "album.flac"}, {Name: "album.cue"}}
	plan, err := BuildSplitPlan("/data/in/album", sheet, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(plan.CuePath, filepath.Join("album", "album.cue")) {
		t.Fatalf("cuePath=%q", plan.CuePath)
	}
}

func TestBuildSplitPlan_RejectsMultipleCueFiles(t *testing.T) {
	sheet := CueSheet{File: "album.flac"}
	entries := []DirEntry{
		{Name: "album.flac"},
		{Name: "disc-1.cue"},
		{Name: "disc-2.CUE"},
	}
	_, err := BuildSplitPlan("/music/album", sheet, entries)
	if !errors.Is(err, ErrAmbiguousCue) {
		t.Fatalf("got %v, want ErrAmbiguousCue", err)
	}
}
