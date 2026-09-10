package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTPAddr  string   `yaml:"http_addr"`
	DataDir   string   `yaml:"data_dir"`
	WatchDirs []string `yaml:"watch_dirs"`
	OutDir    string   `yaml:"out_dir"`
	InPlace   bool     `yaml:"in_place"`
	Engine    string   `yaml:"engine"`
	LogLevel  string   `yaml:"log_level"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	applyEnv(&cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("CUEARR_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("CUEARR_WATCH_DIRS"); v != "" {
		cfg.WatchDirs = splitCSV(v)
	}
	if v := os.Getenv("CUEARR_OUT_DIR"); v != "" {
		cfg.OutDir = v
	}
	if v := os.Getenv("CUEARR_ENGINE"); v != "" {
		cfg.Engine = v
	}
	if v := os.Getenv("CUEARR_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("CUEARR_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
