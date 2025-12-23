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
	City     string       `yaml:"city,omitempty"`
	Area     string       `yaml:"area,omitempty"`
	Areas    []AreaConfig `yaml:"areas,omitempty"`
	CacheDir string       `yaml:"cache_dir"`
	CacheTTL string       `yaml:"cache_ttl"`
}

// AreaConfig is one target: either a whole city or a specific area.
type AreaConfig struct {
	City string `yaml:"city,omitempty"`
	Area string `yaml:"area,omitempty"`
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

// loadConfig returns an empty config when the file is missing.
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
		return nil, fmt.Errorf("could not read config (%s): %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config (%s): %w", path, err)
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *Config) error {
	if path == "" {
		path = defaultConfigPath()
	}
	if path == "" {
		return errors.New("no config path available")
	}

	path = expandHome(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not serialize config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("could not write config (%s): %w", path, err)
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

// configBaseDir picks a platform-appropriate config directory.
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

func mergeOptions(cfg *Config, flags Flags) (Options, error) {
	opts := Options{
		CacheDir: firstNonEmpty(flags.CacheDir, cfg.CacheDir, defaultCacheDir()),
		Name:     strings.TrimSpace(flags.Name),
		Search:   strings.TrimSpace(flags.Search),
		Menu:     strings.TrimSpace(flags.Menu),
	}

	if len(flags.Areas) > 0 {
		if strings.TrimSpace(flags.City) == "" {
			return opts, errors.New("city must be provided when using --area")
		}
		opts.Areas = makeAreas(flags.City, flags.Areas)
	} else if strings.TrimSpace(flags.City) != "" {
		opts.Areas = []AreaConfig{{City: strings.TrimSpace(flags.City)}}
	} else {
		opts.Areas = configAreas(cfg)
	}

	if len(opts.Areas) == 0 {
		return opts, errors.New("city and area must be provided via flags or config")
	}

	// cache_ttl accepts either a full duration (6h) or just hours (6).
	if ttlStr := firstNonEmpty(flags.CacheTTL, cfg.CacheTTL, "6h"); ttlStr != "" {
		dur, ok := parseCacheTTL(ttlStr)
		if ok {
			opts.CacheTTL = dur
		} else if flags.CacheTTL != "" {
			return opts, fmt.Errorf("invalid --cache-ttl %q (use e.g. 6h, 1h, 48h)", flags.CacheTTL)
		} else {
			opts.CacheTTL = 6 * time.Hour
		}
	}

	return opts, nil
}

func parseCacheTTL(input string) (time.Duration, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, false
	}
	if dur, err := time.ParseDuration(input); err == nil {
		return dur, true
	}
	if allDigits(input) {
		if hours, err := time.ParseDuration(input + "h"); err == nil {
			return hours, true
		}
	}
	return 0, false
}

func allDigits(input string) bool {
	for _, r := range input {
		if r < '0' || r > '9' {
			return false
		}
	}
	return input != ""
}

func configAreas(cfg *Config) []AreaConfig {
	if cfg == nil {
		return nil
	}
	defaultCity := strings.TrimSpace(cfg.City)
	var areas []AreaConfig
	for _, area := range cfg.Areas {
		city := strings.TrimSpace(area.City)
		if city == "" {
			city = defaultCity
		}
		areaSlug := strings.TrimSpace(area.Area)
		if city == "" {
			continue
		}
		areas = append(areas, AreaConfig{City: city, Area: areaSlug})
	}
	if len(areas) == 0 && defaultCity != "" {
		areas = append(areas, AreaConfig{City: defaultCity, Area: strings.TrimSpace(cfg.Area)})
	}
	return areas
}

func makeAreas(city string, areas []string) []AreaConfig {
	var targets []AreaConfig
	for _, area := range areas {
		area = strings.TrimSpace(area)
		if area == "" {
			continue
		}
		targets = append(targets, AreaConfig{City: strings.TrimSpace(city), Area: area})
	}
	return targets
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
