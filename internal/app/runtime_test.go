package app_test

import (
	"context"
	"testing"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

type runtimeSettingsStore struct {
	settings domain.Settings
	puts     int
}

func (s *runtimeSettingsStore) Get(context.Context) (domain.Settings, error) {
	return s.settings, nil
}

func (s *runtimeSettingsStore) Put(_ context.Context, settings domain.Settings) error {
	s.settings = settings
	s.puts++
	return nil
}

func TestRuntimeConfig_ApplyChangesWorkerSettings(t *testing.T) {
	first := &fakeSplitter{}
	runtime := app.NewRuntimeConfig(domain.Settings{
		WatchDirs: []string{"/old/watch"},
		OutDir:    "/old/out",
		Engine:    "shntool",
	}, first)

	next := domain.Settings{
		WatchDirs: []string{"/new/watch"},
		OutDir:    "/new/out",
		InPlace:   true,
		Engine:    "native",
	}
	second := &fakeSplitter{}
	runtime.Apply(next, second)

	got, splitter := runtime.Snapshot()
	if got.Engine != "native" || got.OutDir != "/new/out" || !got.InPlace {
		t.Fatalf("settings=%+v", got)
	}
	if splitter != second {
		t.Fatal("splitter was not replaced")
	}
}

func TestLoadRuntimeSettings_SeedsDefaultsOnlyWhenEmpty(t *testing.T) {
	defaults := domain.Settings{WatchDirs: []string{"/yaml"}, OutDir: "/yaml/out", Engine: "shntool"}
	store := &runtimeSettingsStore{}
	got, seeded, err := app.LoadRuntimeSettings(context.Background(), store, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if !seeded || store.puts != 1 || got.OutDir != defaults.OutDir {
		t.Fatalf("got=%+v seeded=%v puts=%d", got, seeded, store.puts)
	}

	store.settings = domain.Settings{WatchDirs: []string{"/db"}, OutDir: "/db/out", Engine: "native"}
	store.puts = 0
	got, seeded, err = app.LoadRuntimeSettings(context.Background(), store, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if seeded || store.puts != 0 || got.OutDir != "/db/out" {
		t.Fatalf("got=%+v seeded=%v puts=%d", got, seeded, store.puts)
	}
}
