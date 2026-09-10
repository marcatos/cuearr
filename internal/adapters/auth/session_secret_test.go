package auth_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/auth"
)

func TestLoadOrCreateSessionSecret_Persists(t *testing.T) {
	dir := t.TempDir()
	a, err := auth.LoadOrCreateSessionSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 32 {
		t.Fatalf("len=%d", len(a))
	}
	path := filepath.Join(dir, "session.key")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	b, err := auth.LoadOrCreateSessionSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("expected same secret on second load")
	}
}
