package domain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCue_Basic(t *testing.T) {
	data, err := os.ReadFile("testdata/cue/basic.cue")
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := ParseCue(data)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Title != "Demo Album" {
		t.Fatalf("title=%q", sheet.Title)
	}
	if sheet.File != "album.flac" {
		t.Fatalf("file=%q", sheet.File)
	}
	if len(sheet.Tracks) != 2 {
		t.Fatalf("tracks=%d", len(sheet.Tracks))
	}
	if sheet.Tracks[0].Index01 != "00:00:00" {
		t.Fatalf("idx=%q", sheet.Tracks[0].Index01)
	}
}

func TestParseCue_QuotedFileNameWithSpaces(t *testing.T) {
	sheet, err := ParseCue([]byte(`FILE "Artist - Album.flac" WAVE`))
	if err != nil {
		t.Fatal(err)
	}
	if sheet.File != "Artist - Album.flac" {
		t.Fatalf("file=%q", sheet.File)
	}
}

func TestBuildSplitPlan_FindsFlacBesideCue(t *testing.T) {
	sheet := CueSheet{File: "album.flac", Tracks: []CueTrack{{Number: 1, Title: "A", Index01: "00:00:00"}}}
	entries := []DirEntry{{Name: "album.flac"}, {Name: "album.cue"}}
	plan, err := BuildSplitPlan("/data/in/album", sheet, entries)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ImagePath != filepath.Join("/data/in/album", "album.flac") {
		t.Fatal(plan.ImagePath)
	}
}

func TestBuildSplitPlan_MissingImage(t *testing.T) {
	sheet := CueSheet{File: "missing.flac", Tracks: []CueTrack{{Number: 1}}}
	_, err := BuildSplitPlan("/tmp", sheet, nil)
	if !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("got %v", err)
	}
}
