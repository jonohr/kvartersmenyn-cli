package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func defaultCacheDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Caches", "kvartersmenyn")
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.Getenv("TEMP")
		}
		if base == "" && home != "" {
			base = filepath.Join(home, "AppData", "Local", "Temp")
		}
		if base == "" {
			return ""
		}
		return filepath.Join(base, "kvartersmenyn", "Cache")
	default:
		base := os.Getenv("XDG_CACHE_HOME")
		if base == "" && home != "" {
			base = filepath.Join(home, ".cache")
		}
		if base == "" {
			return ""
		}
		return filepath.Join(base, "kvartersmenyn")
	}
}

func defaultConfigPath() string {
	base := configBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "config.yaml")
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

func saveConfig(path string, cfg *Config) error {
	if path == "" {
		path = defaultConfigPath()
	}
	if path == "" {
		return errors.New("ingen config-sökväg tillgänglig")
	}

	path = expandHome(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("kunde inte skapa konfigkatalog: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("kunde inte serialisera config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("kunde inte skriva config (%s): %w", path, err)
	}
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func configBaseDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support", "kvartersmenyn")
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.Getenv("APPDATA")
		}
		if base == "" && home != "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
		if base == "" {
			return ""
		}
		return filepath.Join(base, "kvartersmenyn")
	default:
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" && home != "" {
			base = filepath.Join(home, ".config")
		}
		if base == "" {
			return ""
		}
		return filepath.Join(base, "kvartersmenyn")
	}
}

func mergeOptions(cfg *Config, flags Flags) Options {
	opts := Options{
		City:     firstNonEmpty(flags.City, cfg.City),
		Area:     firstNonEmpty(flags.Area, cfg.Area),
		CacheDir: firstNonEmpty(flags.CacheDir, cfg.CacheDir, defaultCacheDir()),
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
