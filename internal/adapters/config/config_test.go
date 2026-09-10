package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/config"
)

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cuearr.yaml")
	const yaml = `http_addr: ":1111"
data_dir: ./from-yaml
watch_dirs: ["/yaml/watch"]
out_dir: "/yaml/out"
in_place: true
engine: native
log_level: debug
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CUEARR_DATA_DIR", "/env/data")
	t.Setenv("CUEARR_WATCH_DIRS", "/env/a,/env/b")
	t.Setenv("CUEARR_OUT_DIR", "/env/out")
	t.Setenv("CUEARR_ENGINE", "shntool")
	t.Setenv("CUEARR_HTTP_ADDR", ":9999")
	t.Setenv("CUEARR_LOG_LEVEL", "warn")
	t.Setenv("CUEARR_COOKIE_SECURE", "true")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("HTTPAddr=%q", cfg.HTTPAddr)
	}
	if cfg.DataDir != "/env/data" {
		t.Fatalf("DataDir=%q", cfg.DataDir)
	}
	if len(cfg.WatchDirs) != 2 || cfg.WatchDirs[0] != "/env/a" || cfg.WatchDirs[1] != "/env/b" {
		t.Fatalf("WatchDirs=%v", cfg.WatchDirs)
	}
	if cfg.OutDir != "/env/out" {
		t.Fatalf("OutDir=%q", cfg.OutDir)
	}
	if !cfg.InPlace {
		t.Fatal("InPlace=false, want true from yaml (not overridden)")
	}
	if cfg.Engine != "shntool" {
		t.Fatalf("Engine=%q", cfg.Engine)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel=%q", cfg.LogLevel)
	}
	if !cfg.CookieSecure {
		t.Fatal("CookieSecure=false")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error")
	}
}
