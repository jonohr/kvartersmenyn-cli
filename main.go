package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

type Flags struct {
	City     string
	Area     string
	File     string
	Search   string
	CacheDir string
	CacheTTL string
	Config   string
}

type Options struct {
	City     string
	Area     string
	File     string
	Search   string
	CacheDir string
	CacheTTL time.Duration
}

func main() {
	flags := Flags{}
	flag.StringVar(&flags.City, "city", "", "City segment used in the kvartersmenyn URL (can be set in config)")
	flag.StringVar(&flags.Area, "area", "", "Area slug from kvartersmenyn, e.g. garda_161 (can be set in config)")
	flag.StringVar(&flags.File, "file", "", "Optional local HTML file to parse instead of fetching from the site")
	flag.StringVar(&flags.Search, "search", "", "Filter by restaurant name (fuzzy, case-insensitive)")
	flag.StringVar(&flags.CacheDir, "cache-dir", "", "Directory for cached HTML (empty to disable, can be set in config)")
	flag.StringVar(&flags.CacheTTL, "cache-ttl", "", "How long to reuse cached HTML (e.g. 6h, 2h). Overwrites config/default when set.")
	flag.StringVar(&flags.Config, "config", defaultConfigPath(), "Path to YAML config (city, area, cache)")
	flag.Parse()

	cfg, err := loadConfig(flags.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}

	opts := mergeOptions(cfg, flags)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var reader io.ReadCloser
	var sourceDesc string
	if opts.File != "" {
		file, err := os.Open(opts.File)
		if err != nil {
			log.Fatalf("kunde inte läsa filen %s: %v", opts.File, err)
		}
		reader = file
		sourceDesc = opts.File
	} else {
		if opts.City == "" || opts.Area == "" {
			log.Fatal("city och area måste anges via flaggor eller config")
		}
		if cache, desc, ok := tryCache(opts.CacheDir, opts.City, opts.Area, opts.CacheTTL); ok {
			reader = cache
			sourceDesc = desc
		} else {
			url := buildAreaURL(opts.City, opts.Area)
			resp, err := fetchHTML(ctx, url)
			if err != nil {
				log.Fatalf("kunde inte hämta data: %v", err)
			}
			reader, sourceDesc = cacheAndWrap(resp.Body, url, opts.CacheDir, opts.City, opts.Area)
		}
	}
	defer reader.Close()

	restaurants, err := parseRestaurants(reader)
	if err != nil {
		log.Fatalf("kunde inte tolka sidan: %v", err)
	}

	query := strings.TrimSpace(opts.Search)
	if query != "" {
		restaurants = filterRestaurants(restaurants, query)
	}

	if len(restaurants) == 0 {
		if query != "" {
			fmt.Printf("Inga träffar på \"%s\" i %s\n", query, sourceDesc)
		} else {
			fmt.Printf("Hittade inga lunchmenyer i %s\n", sourceDesc)
		}
		return
	}

	if query != "" {
		fmt.Printf("Lunchmenyer från %s (sök: %s)\n\n", sourceDesc, query)
	} else {
		fmt.Printf("Lunchmenyer från %s\n\n", sourceDesc)
	}
	for _, r := range restaurants {
		fmt.Printf("%s — %s\n", r.Name, r.Price)
		if r.Address != "" {
			fmt.Printf("  %s\n", r.Address)
		}
		if r.Phone != "" {
			fmt.Printf("  Tel: %s\n", r.Phone)
		}
		if r.Link != "" {
			fmt.Printf("  Länk: %s\n", r.Link)
		}
		if len(r.Menu) > 0 {
			fmt.Printf("  Meny:\n")
			for _, line := range r.Menu {
				fmt.Printf("    - %s\n", line)
			}
		}
		fmt.Println()
	}
}

func buildAreaURL(city, area string) string {
	return fmt.Sprintf("https://www.kvartersmenyn.se/index.php/%s/area/%s", city, area)
}

func fetchHTML(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "kvartersmenyn-cli/0.1 (+https://www.kvartersmenyn.se/)")
	req.Header.Set("Accept-Language", "sv-SE,sv;q=0.9,en;q=0.8")

	client := http.Client{
		Timeout: 12 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("oväntad statuskod %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func tryCache(dir, city, area string, ttl time.Duration) (io.ReadCloser, string, bool) {
	if dir == "" || ttl <= 0 {
		return nil, "", false
	}
	cachePath := filepath.Join(dir, fmt.Sprintf("%s_%s.html", city, area))
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, "", false
	}
	if time.Since(info.ModTime()) > ttl {
		return nil, "", false
	}
	file, err := os.Open(cachePath)
	if err != nil {
		return nil, "", false
	}
	return file, "cache:" + cachePath, true
}

func cacheAndWrap(body io.ReadCloser, url, dir, city, area string) (io.ReadCloser, string) {
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		log.Fatalf("kunde inte läsa svaret: %v", err)
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			cachePath := filepath.Join(dir, fmt.Sprintf("%s_%s.html", city, area))
			if err := os.WriteFile(cachePath, data, 0o644); err != nil {
				log.Printf("kunde inte skriva cache (%s): %v", cachePath, err)
			}
		} else {
			log.Printf("kunde inte skapa cachekatalog (%s): %v", dir, err)
		}
	}

	return io.NopCloser(bytes.NewReader(data)), url
}

func filterRestaurants(restaurants []Restaurant, query string) []Restaurant {
	queryLower := strings.ToLower(query)
	maxDistance := fuzzThreshold(len(query))

	var filtered []Restaurant
	for _, r := range restaurants {
		if matchesName(r.Name, queryLower, maxDistance) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func matchesName(name, queryLower string, maxDistance int) bool {
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, queryLower) {
		return true
	}

	dist := fuzzy.RankMatchFold(queryLower, lowerName)
	return dist >= 0 && dist <= maxDistance
}

func fuzzThreshold(length int) int {
	if length <= 3 {
		return 1
	}
	if length <= 6 {
		return 2
	}
	return 3
}
