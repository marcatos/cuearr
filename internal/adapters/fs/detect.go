package fs

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultScanDepth = 3

func IsCueSheet(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".cue")
}

func IsLosslessImage(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".flac") ||
		strings.HasSuffix(lower, ".wav") ||
		strings.HasSuffix(lower, ".ape") ||
		strings.HasSuffix(lower, ".wv") ||
		strings.HasSuffix(lower, ".tta")
}

func IsWatchTarget(name string) bool {
	return IsCueSheet(name) || IsLosslessImage(name)
}

func ResolveAlbumDir(eventPath string) string {
	return filepath.Clean(filepath.Dir(eventPath))
}

func WalkCueDirs(roots []string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		maxDepth = DefaultScanDepth
	}
	seen := make(map[string]struct{})
	var out []string
	for _, root := range roots {
		root = filepath.Clean(root)
		if err := walkCueDirs(root, 0, maxDepth, seen, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func walkCueDirs(dir string, depth, maxDepth int, seen map[string]struct{}, out *[]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && IsCueSheet(e.Name()) {
			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				*out = append(*out, dir)
			}
			break
		}
	}
	if depth >= maxDepth {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		if err := walkCueDirs(child, depth+1, maxDepth, seen, out); err != nil {
			return err
		}
	}
	return nil
}
