package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	City     string `yaml:"city"`
	Area     string `yaml:"area"`
	CacheDir string `yaml:"cache_dir"`
	CacheTTL string `yaml:"cache_ttl"`
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "kvartersmenyn", "config.yaml")
}

func loadConfig(path string) (*Config, error) {
	if path == "" {
		return &Config{}, nil
	}
	path = expandHome(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("kunde inte läsa config (%s): %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("kunde inte tolka config (%s): %w", path, err)
	}
	return &cfg, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func mergeOptions(cfg *Config, flags Flags) Options {
	opts := Options{
		City:     firstNonEmpty(flags.City, cfg.City),
		Area:     firstNonEmpty(flags.Area, cfg.Area),
		CacheDir: firstNonEmpty(flags.CacheDir, cfg.CacheDir),
		Search:   strings.TrimSpace(flags.Search),
		File:     flags.File,
	}

	if ttlStr := firstNonEmpty(flags.CacheTTL, cfg.CacheTTL, "6h"); ttlStr != "" {
		dur, err := time.ParseDuration(ttlStr)
		if err == nil {
			opts.CacheTTL = dur
		}
	}

	return opts
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
