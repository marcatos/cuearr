package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DirEntry struct {
	Name string
}

type FileStat struct {
	Size            int64
	ModTimeUnixNano int64
}

type ImageIdentity struct {
	Size            int64
	ModTimeUnixNano int64
	ContentSHA256   string
}

type SplitPlan struct {
	CuePath, ImagePath, WorkDir string
	Tracks                      []CueTrack
	Fingerprint                 string
}

// HashFile, when set, replaces full-file SHA-256 hashing (tests).
var HashFile func(path string) (hexSHA256 string, err error)

func HashFileContent(path string) (string, error) {
	if HashFile != nil {
		return HashFile(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func Fingerprint(cuePath, imagePath string, cueBytes []byte, img ImageIdentity) string {
	cueHash := sha256.Sum256(cueBytes)
	cueHex := hex.EncodeToString(cueHash[:])
	payload := fmt.Sprintf("%s|%s|%s|%d|%d|%s",
		cuePath, imagePath, cueHex, img.Size, img.ModTimeUnixNano, img.ContentSHA256)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func BuildSplitPlan(dir string, sheet CueSheet, entries []DirEntry) (SplitPlan, error) {
	imageName := sheet.File
	if imageName == "" {
		imageName = pickSingleAudio(entries)
	}
	if imageName == "" || !entryExists(entries, imageName) {
		return SplitPlan{}, ErrImageNotFound
	}

	cueName := pickCueFile(entries)
	cuePath := ""
	if cueName != "" {
		cuePath = filepath.Join(dir, cueName)
	}

	plan := SplitPlan{
		CuePath:   cuePath,
		ImagePath: filepath.Join(dir, imageName),
		WorkDir:   dir,
		Tracks:    append([]CueTrack(nil), sheet.Tracks...),
	}
	return plan, nil
}

func pickCueFile(entries []DirEntry) string {
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name), ".cue") {
			return e.Name
		}
	}
	return ""
}

func entryExists(entries []DirEntry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

func pickSingleAudio(entries []DirEntry) string {
	var candidates []string
	for _, e := range entries {
		lower := strings.ToLower(e.Name)
		if strings.HasSuffix(lower, ".flac") || strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".ape") {
			candidates = append(candidates, e.Name)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}
