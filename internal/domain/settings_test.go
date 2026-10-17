package domain_test

import (
	"testing"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestSettings_MaxAttemptsDefaultsToThreeTotalAttempts(t *testing.T) {
	if got := (domain.Settings{}).MaxAttempts(); got != 3 {
		t.Fatalf("MaxAttempts()=%d want 3", got)
	}
}

func TestSettings_MaxAttemptsUsesConfiguredMaxRetries(t *testing.T) {
	settings := domain.Settings{
		MaxRetries:            5,
		LidarrURL:             "http://lidarr:8686",
		LidarrAPIKey:          "secret",
		LidarrImportEnabled:   true,
		LidarrPathMap:         []domain.PathMapRule{{From: "/downloads", To: "/music"}},
		LidarrPollIntervalSec: 60,
	}
	if got := settings.MaxAttempts(); got != 5 {
		t.Fatalf("MaxAttempts()=%d want 5", got)
	}
}

func TestMapLidarrPathAppliesFirstMatchingPrefix(t *testing.T) {
	rules := []domain.PathMapRule{
		{From: "/downloads", To: "/lidarr-downloads"},
		{From: "/downloads/music", To: "/music"},
	}

	got := domain.MapLidarrPath("/downloads/music/album", rules)

	if got != "/lidarr-downloads/music/album" {
		t.Fatalf("MapLidarrPath() = %q, want %q", got, "/lidarr-downloads/music/album")
	}
}

func TestMapLidarrPathReturnsUnmatchedPath(t *testing.T) {
	const path = "/music/artist/album"

	if got := domain.MapLidarrPath(path, []domain.PathMapRule{{From: "/downloads", To: "/imports"}}); got != path {
		t.Fatalf("MapLidarrPath() = %q, want %q", got, path)
	}
}

func TestReview_MapLidarrPathRespectsDirectoryBoundary(t *testing.T) {
	const path = "/downloads-other/artist/album"
	rules := []domain.PathMapRule{{From: "/downloads", To: "/imports"}}
	if got := domain.MapLidarrPath(path, rules); got != path {
		t.Fatalf("sibling directory incorrectly remapped: got %q, want %q", got, path)
	}
}

func TestMapLidarrPathNormalizesRuleSeparators(t *testing.T) {
	for _, tt := range []struct{ path, from, to, want string }{
		{"/downloads/album", "/downloads/", "/imports/", "/imports/album"},
		{"/album", "/", "/imports", "/imports/album"},
		{`C:\downloads\album`, `C:\downloads\`, `D:\imports\`, `D:\imports\album`},
		{"/album", "", "/imports", "/album"},
	} {
		if got := domain.MapLidarrPath(tt.path, []domain.PathMapRule{{From: tt.from, To: tt.to}}); got != tt.want {
			t.Errorf("mapping %q: got %q, want %q", tt.path, got, tt.want)
		}
	}
}
