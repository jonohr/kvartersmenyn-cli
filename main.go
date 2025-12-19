package main

import (
	"bufio"
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
	"unicode"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

type Flags struct {
	City     string
	Area     string
	File     string
	Name     string
	Search   string
	Menu     string
	CacheDir string
	CacheTTL string
	Config   string
	Help     bool
}

type Options struct {
	City     string
	Area     string
	File     string
	Name     string
	Search   string
	Menu     string
	CacheDir string
	CacheTTL time.Duration
}

func main() {
	flags := Flags{}
	flag.StringVar(&flags.City, "city", "", "City segment used in the kvartersmenyn URL (can be set in config)")
	flag.StringVar(&flags.Area, "area", "", "Area slug from kvartersmenyn, e.g. garda_161 (can be set in config)")
	flag.StringVar(&flags.File, "file", "", "Optional local HTML file to parse instead of fetching from the site")
	flag.StringVar(&flags.Name, "name", "", "Filter by restaurant name (fuzzy, case-insensitive)")
	flag.StringVar(&flags.Menu, "menu", "", "Filter by menu text (fuzzy, case-insensitive)")
	flag.StringVar(&flags.Search, "search", "", "Filter both name and menu (fuzzy, case-insensitive)")
	flag.StringVar(&flags.CacheDir, "cache-dir", "", "Directory for cached HTML (empty to disable, can be set in config)")
	flag.StringVar(&flags.CacheTTL, "cache-ttl", "", "How long to reuse cached HTML (e.g. 6h, 2h). Overwrites config/default when set.")
	flag.StringVar(&flags.Config, "config", defaultConfigPath(), "Path to YAML config (city, area, cache)")
	flag.BoolVar(&flags.Help, "help", false, "Show help")
	flag.Parse()

	if flags.Help {
		flag.Usage()
		return
	}

	cfg, err := loadConfig(flags.Config)
	if err != nil || cfg == nil || cfg.City == "" || cfg.Area == "" {
		fmt.Println("No valid config found. We need a kvartersmenyn URL and (optional) cache TTL.")
		cfg = promptAndSaveConfig(flags.Config)
	}

	opts := mergeOptions(cfg, flags)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var reader io.ReadCloser
	var sourceDesc string
	if opts.File != "" {
		file, err := os.Open(opts.File)
		if err != nil {
			log.Fatalf("could not read file %s: %v", opts.File, err)
		}
		reader = file
		sourceDesc = opts.File
	} else {
		if opts.City == "" || opts.Area == "" {
			log.Fatal("city and area must be provided via flags or config")
		}
		if cache, desc, ok := tryCache(opts.CacheDir, opts.City, opts.Area, opts.CacheTTL); ok {
			reader = cache
			sourceDesc = desc
		} else {
			url := buildAreaURL(opts.City, opts.Area)
			resp, err := fetchHTML(ctx, url)
			if err != nil {
				log.Fatalf("could not fetch data: %v", err)
			}
			reader, sourceDesc = cacheAndWrap(resp.Body, url, opts.CacheDir, opts.City, opts.Area)
		}
	}
	defer reader.Close()

	restaurants, err := parseRestaurants(reader)
	if err != nil {
		log.Fatalf("could not parse page: %v", err)
	}

	nameQuery := strings.TrimSpace(opts.Name)
	menuQuery := strings.TrimSpace(opts.Menu)
	combinedQuery := strings.TrimSpace(opts.Search)

	if combinedQuery != "" {
		if nameQuery == "" {
			nameQuery = combinedQuery
		}
		if menuQuery == "" {
			menuQuery = combinedQuery
		}
		restaurants = filterCombined(restaurants, nameQuery, menuQuery)
	} else {
		if nameQuery != "" {
			restaurants = filterRestaurants(restaurants, nameQuery)
		}
		if menuQuery != "" {
			restaurants = filterByMenu(restaurants, menuQuery)
		}
	}

	if len(restaurants) == 0 {
		noHitMsg(sourceDesc, nameQuery, menuQuery)
		return
	}

	printHeader(sourceDesc, nameQuery, menuQuery)
	for _, r := range restaurants {
		fmt.Printf("%s — %s\n", r.Name, r.Price)
		if r.Address != "" {
			fmt.Printf("  %s\n", r.Address)
		}
		if r.Phone != "" {
			fmt.Printf("  Tel: %s\n", r.Phone)
		}
		if r.Link != "" {
			fmt.Printf("  Link: %s\n", r.Link)
		}
		if len(r.Menu) > 0 {
			fmt.Printf("  Menu:\n")
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
		log.Fatalf("could not read response body: %v", err)
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			cachePath := filepath.Join(dir, fmt.Sprintf("%s_%s.html", city, area))
			if err := os.WriteFile(cachePath, data, 0o644); err != nil {
				log.Printf("could not write cache (%s): %v", cachePath, err)
			}
		} else {
			log.Printf("could not create cache directory (%s): %v", dir, err)
		}
	}

	return io.NopCloser(bytes.NewReader(data)), url
}

func promptAndSaveConfig(path string) *Config {
	reader := bufio.NewReader(os.Stdin)

	var city, area string
	for {
		fmt.Print("Enter kvartersmenyn URL (e.g. https://www.kvartersmenyn.se/index.php/goteborg/area/garda_161): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		c, a, ok := parseAreaURL(line)
		if !ok {
			fmt.Println("Could not parse the URL. Please try again.")
			continue
		}
		city, area = c, a
		break
	}

	fmt.Print("Cache TTL in Go duration format (default 6h): ")
	ttlInput, _ := reader.ReadString('\n')
	ttlInput = strings.TrimSpace(ttlInput)
	if ttlInput == "" {
		ttlInput = "6h"
	}

	cacheDir := defaultCacheDir()
	if cacheDir == "" {
		cacheDir = ".cache"
	}

	cfg := &Config{
		City:     city,
		Area:     area,
		CacheDir: cacheDir,
		CacheTTL: ttlInput,
	}

	if err := saveConfig(path, cfg); err != nil {
		fmt.Printf("Warning: could not write config: %v\n", err)
	}

	return cfg
}

func parseAreaURL(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}

	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")

	if idx := strings.Index(raw, "kvartersmenyn.se/"); idx >= 0 {
		raw = raw[idx+len("kvartersmenyn.se/"):]
	}
	if idx := strings.Index(raw, "index.php/"); idx >= 0 {
		raw = raw[idx+len("index.php/"):]
	}

	parts := strings.Split(raw, "/")
	if len(parts) < 3 {
		return "", "", false
	}

	city := parts[0]
	var area string
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "area" {
			area = parts[i+1]
			break
		}
	}

	if city == "" || area == "" {
		return "", "", false
	}

	return city, area, true
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

	normName := normalizeToken(lowerName)
	normQuery := normalizeToken(queryLower)

	if normQuery != "" && strings.Contains(normName, normQuery) {
		return true
	}

	dist := fuzzy.RankMatchFold(normQuery, normName)
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

func normalizeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func filterByMenu(restaurants []Restaurant, query string) []Restaurant {
	queryLower := strings.ToLower(query)
	normQuery := normalizeToken(queryLower)
	maxDistance := fuzzThreshold(len(normQuery))

	var filtered []Restaurant
	for _, r := range restaurants {
		menuText := strings.ToLower(strings.Join(r.Menu, " "))
		if matchesText(menuText, queryLower, normQuery, maxDistance) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func matchesText(text, rawQuery, normQuery string, maxDistance int) bool {
	if strings.Contains(text, rawQuery) {
		return true
	}
	normText := normalizeToken(text)
	if normQuery != "" && strings.Contains(normText, normQuery) {
		return true
	}
	if normQuery == "" {
		return false
	}
	dist := fuzzy.RankMatchFold(normQuery, normText)
	return dist >= 0 && dist <= maxDistance
}

func filterCombined(restaurants []Restaurant, nameQuery, menuQuery string) []Restaurant {
	nameLower := strings.ToLower(strings.TrimSpace(nameQuery))
	menuLower := strings.ToLower(strings.TrimSpace(menuQuery))

	normName := normalizeToken(nameLower)
	normMenu := normalizeToken(menuLower)

	maxName := fuzzThreshold(len(normName))
	maxMenu := fuzzThreshold(len(normMenu))

	var filtered []Restaurant
	for _, r := range restaurants {
		matchedName := false
		matchedMenu := false

		if nameLower != "" {
			matchedName = matchesName(r.Name, nameLower, maxName)
		}
		if menuLower != "" {
			menuText := strings.ToLower(strings.Join(r.Menu, " "))
			matchedMenu = matchesText(menuText, menuLower, normMenu, maxMenu)
		}

		if matchedName || matchedMenu {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func noHitMsg(source, nameQuery, menuQuery string) {
	switch {
	case nameQuery != "" && menuQuery != "":
		fmt.Printf("No matches for name \"%s\" or menu \"%s\" in %s\n", nameQuery, menuQuery, source)
	case nameQuery != "":
		fmt.Printf("No matches for \"%s\" in %s\n", nameQuery, source)
	case menuQuery != "":
		fmt.Printf("No menu lines matched \"%s\" in %s\n", menuQuery, source)
	default:
		fmt.Printf("No lunch menus found in %s\n", source)
	}
}

func printHeader(source, nameQuery, menuQuery string) {
	switch {
	case nameQuery != "" && menuQuery != "":
		fmt.Printf("Lunch menus from %s (name: %s, menu: %s)\n\n", source, nameQuery, menuQuery)
	case nameQuery != "":
		fmt.Printf("Lunch menus from %s (name: %s)\n\n", source, nameQuery)
	case menuQuery != "":
		fmt.Printf("Lunch menus from %s (menu: %s)\n\n", source, menuQuery)
	default:
		fmt.Printf("Lunch menus from %s\n\n", source)
	}
}
