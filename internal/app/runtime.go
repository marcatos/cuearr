package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

func LoadRuntimeSettings(
	ctx context.Context,
	store ports.SettingsStore,
	defaults domain.Settings,
) (domain.Settings, bool, error) {
	settings, err := store.Get(ctx)
	if err != nil {
		return domain.Settings{}, false, fmt.Errorf("load runtime settings: %w", err)
	}
	if !runtimeSettingsEmpty(settings) {
		return settings, false, nil
	}
	defaults.Auth = settings.Auth
	if err := store.Put(ctx, defaults); err != nil {
		return domain.Settings{}, false, fmt.Errorf("seed runtime settings: %w", err)
	}
	return defaults, true, nil
}

func runtimeSettingsEmpty(settings domain.Settings) bool {
	return len(settings.WatchDirs) == 0 && settings.OutDir == "" &&
		!settings.InPlace && settings.Engine == ""
}

// RuntimeConfig holds daemon settings and adapters that can change at runtime.
type RuntimeConfig struct {
	mu       sync.RWMutex
	settings domain.Settings
	splitter ports.Splitter
}

func NewRuntimeConfig(settings domain.Settings, splitter ports.Splitter) *RuntimeConfig {
	return &RuntimeConfig{settings: cloneSettings(settings), splitter: splitter}
}

func (c *RuntimeConfig) Snapshot() (domain.Settings, ports.Splitter) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneSettings(c.settings), c.splitter
}

func (c *RuntimeConfig) Apply(settings domain.Settings, splitter ports.Splitter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settings = cloneSettings(settings)
	c.splitter = splitter
}

func cloneSettings(settings domain.Settings) domain.Settings {
	settings.WatchDirs = append([]string(nil), settings.WatchDirs...)
	settings.Auth.OIDCAllowedEmailDomains = append(
		[]string(nil), settings.Auth.OIDCAllowedEmailDomains...,
	)
	return settings
}
