package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestFingerprint_Deterministic(t *testing.T) {
	cueBytes := []byte("FILE \"a.flac\" WAVE\n")
	got := Fingerprint("/album/x.cue", "/album/a.flac", cueBytes)
	cueHash := sha256.Sum256(cueBytes)
	inner := hex.EncodeToString(cueHash[:])
	payload := "/album/x.cue|/album/a.flac|" + inner
	wantHash := sha256.Sum256([]byte(payload))
	want := hex.EncodeToString(wantHash[:])
	if got != want {
		t.Fatalf("fingerprint=%q want=%q", got, want)
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
