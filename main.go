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
	Areas    areaList
	Name     string
	Search   string
	Menu     string
	Day      string
	CacheDir string
	CacheTTL string
	Config   string
	Help     bool
	InitCfg  bool
}

type Options struct {
	Areas    []AreaConfig
	Name     string
	Search   string
	Menu     string
	Day      int
	CacheDir string
	CacheTTL time.Duration
}

type areaList []string

func (a *areaList) String() string {
	return strings.Join(*a, ",")
}

func (a *areaList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*a = append(*a, part)
		}
	}
	return nil
}

func main() {
	flags := Flags{}
	flag.StringVar(&flags.City, "city", "", "City segment used in the kvartersmenyn URL (can be set in config)")
	flag.Var(&flags.Areas, "area", "Area slug from kvartersmenyn, e.g. garda_161 (can be repeated or comma-separated)")
	flag.StringVar(&flags.Name, "name", "", "Filter by restaurant name (fuzzy, case-insensitive)")
	flag.StringVar(&flags.Menu, "menu", "", "Filter by menu text (fuzzy, case-insensitive)")
	flag.StringVar(&flags.Search, "search", "", "Filter both name and menu (fuzzy, case-insensitive)")
	flag.StringVar(&flags.Day, "day", "", "Day of week to fetch (mon, tue, wed, thu, fri, sat, sun or 1-7)")
	flag.StringVar(&flags.CacheDir, "cache-dir", "", "Directory for cached HTML (empty to disable, can be set in config)")
	flag.StringVar(&flags.CacheTTL, "cache-ttl", "", "How long to reuse cached HTML (e.g. 6h, 2h). Overwrites config/default when set.")
	flag.StringVar(&flags.Config, "config", defaultConfigPath(), "Path to YAML config (city, area, cache)")
	flag.BoolVar(&flags.Help, "help", false, "Show help")
	flag.BoolVar(&flags.InitCfg, "init-config", false, "Run the interactive config setup and exit")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(out, "Options:")
		fmt.Fprintln(out, "  --city        City segment used in the kvartersmenyn URL (can be set in config)")
		fmt.Fprintln(out, "  --area        Area slug from kvartersmenyn, e.g. garda_161 (repeat or comma-separated)")
		fmt.Fprintln(out, "  --name        Filter by restaurant name (fuzzy, case-insensitive)")
		fmt.Fprintln(out, "  --menu        Filter by menu text (fuzzy, case-insensitive)")
		fmt.Fprintln(out, "  --search      Filter both name and menu (fuzzy, case-insensitive)")
		fmt.Fprintln(out, "  --day         Day of week to fetch (mon, tue, wed, thu, fri, sat, sun or 1-7)")
		fmt.Fprintln(out, "  --cache-dir   Directory for cached HTML (empty to disable, can be set in config)")
		fmt.Fprintln(out, "  --cache-ttl   How long to reuse cached HTML (e.g. 6h, 2h)")
		fmt.Fprintf(out, "  --config      Path to YAML config (default: %s)\n", defaultConfigPath())
		fmt.Fprintln(out, "  --init-config Run the interactive config setup and exit")
		fmt.Fprintln(out, "  --help        Show help and exit")
	}
	flag.Parse()

	if flags.Help {
		flag.Usage()
		return
	}

	if flags.InitCfg {
		promptAndSaveConfig(flags.Config)
		return
	}

	cfg, err := loadConfig(flags.Config)
	if err != nil || cfg == nil || len(configAreas(cfg)) == 0 {
		if len(flags.Areas) == 0 {
			fmt.Println("No valid config found. We need at least one kvartersmenyn URL and (optional) cache TTL.")
			promptAndSaveConfig(flags.Config)
			return
		} else if cfg == nil {
			cfg = &Config{}
		}
	}

	opts, err := mergeOptions(cfg, flags)
	if err != nil {
		log.Fatal(err)
	}
	if day, ok := parseDayFlag(flags.Day); ok {
		opts.Day = day
	} else if flags.Day != "" {
		log.Fatalf("invalid --day value: %q (use mon/tue/... or 1-7)", flags.Day)
	} else {
		opts.Day = weekdayToDay(time.Now().Weekday())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	nameQuery := strings.TrimSpace(opts.Name)
	menuQuery := strings.TrimSpace(opts.Menu)
	combinedQuery := strings.TrimSpace(opts.Search)

	for _, area := range opts.Areas {
		reader, sourceRaw, err := loadAreaReader(ctx, opts.CacheDir, area, opts.Day, opts.CacheTTL)
		if err != nil {
			log.Fatalf("could not fetch data for %s: %v", areaLabel(area), err)
		}

		restaurants, err := parseRestaurants(reader)
		reader.Close()
		if err != nil {
			log.Fatalf("could not parse page for %s: %v", areaLabel(area), err)
		}

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

		sourceDesc := describeSource(area, opts.Day, sourceRaw)
		if len(restaurants) == 0 {
			noHitMsg(sourceDesc, nameQuery, menuQuery)
			continue
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
}

func buildAreaURL(city, area string, day int) string {
	if isNumericCity(city) {
		return fmt.Sprintf("https://www.kvartersmenyn.se/index.php/find/_/city/%s/area/%s/day/%d", city, area, day)
	}
	return fmt.Sprintf("https://www.kvartersmenyn.se/index.php/%s/area/%s/day/%d", city, area, day)
}

func buildCityURL(city string, day int) string {
	if isNumericCity(city) {
		return fmt.Sprintf("https://www.kvartersmenyn.se/index.php/find/_/city/%s/day/%d", city, day)
	}
	return fmt.Sprintf("https://www.kvartersmenyn.se/index.php/%s/day/%d", city, day)
}

func areaLabel(area AreaConfig) string {
	if area.Area == "" {
		return area.City
	}
	return fmt.Sprintf("%s/%s", area.City, area.Area)
}

func describeSource(area AreaConfig, day int, source string) string {
	label := areaLabel(area)
	dayLabel := dayLabel(day)
	if dayLabel != "" {
		label = fmt.Sprintf("%s (day %s)", label, dayLabel)
	}
	if strings.HasPrefix(source, "cache:") {
		return label + " (cache)"
	}
	return label
}

func loadAreaReader(ctx context.Context, cacheDir string, area AreaConfig, day int, ttl time.Duration) (io.ReadCloser, string, error) {
	cacheKey := area.Area
	if cacheKey == "" {
		cacheKey = "all"
	}
	cacheKey = fmt.Sprintf("%s_day%d", cacheKey, day)
	if cache, desc, ok := tryCache(cacheDir, area.City, cacheKey, ttl); ok {
		return cache, desc, nil
	}

	var url string
	if area.Area == "" {
		url = buildCityURL(area.City, day)
	} else {
		url = buildAreaURL(area.City, area.Area, day)
	}
	resp, err := fetchHTML(ctx, url)
	if err != nil {
		return nil, "", err
	}
	reader, source := cacheAndWrap(resp.Body, url, cacheDir, area.City, cacheKey)
	return reader, source, nil
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

	var areas []AreaConfig
	var defaultCity string

	addArea := func(city, area string) {
		if defaultCity == "" {
			defaultCity = city
		}
		if city == defaultCity {
			areas = append(areas, AreaConfig{Area: area})
		} else {
			areas = append(areas, AreaConfig{City: city, Area: area})
		}
	}

	askAreaSlug := func(city string) {
		for {
			fmt.Printf("Enter area slug for %s (empty for whole city): ", city)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			addArea(city, line)
			break
		}
	}

	for {
		fmt.Print("Enter kvartersmenyn URL (city or area), e.g. https://www.kvartersmenyn.se/index.php/goteborg or https://www.kvartersmenyn.se/index.php/goteborg/area/garda_161: ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		city, area, ok := parseAreaURL(line)
		if !ok {
			fmt.Println("Could not parse the URL. Please try again.")
			continue
		}
		if area == "" {
			defaultCity = city
			askAreaSlug(city)
		} else {
			addArea(city, area)
		}

		fmt.Print("Add another area? (y/N): ")
		moreInput, _ := reader.ReadString('\n')
		moreInput = strings.TrimSpace(strings.ToLower(moreInput))
		if moreInput != "y" && moreInput != "yes" && moreInput != "j" && moreInput != "ja" {
			break
		}

		for {
			fmt.Print("Enter area slug or kvartersmenyn URL: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if looksLikeURL(line) {
				city, area, ok := parseAreaURL(line)
				if !ok {
					fmt.Println("Could not parse the URL. Please try again.")
					continue
				}
				if area == "" {
					defaultCity = city
					askAreaSlug(city)
				} else {
					addArea(city, area)
				}
				break
			}

			if defaultCity == "" {
				fmt.Println("Please provide a kvartersmenyn URL first to set the city.")
				continue
			}
			addArea(defaultCity, line)
			break
		}
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
		City:     defaultCity,
		Areas:    areas,
		CacheDir: cacheDir,
		CacheTTL: ttlInput,
	}

	if err := saveConfig(path, cfg); err != nil {
		fmt.Printf("Warning: could not write config: %v\n", err)
	}

	return cfg
}

func looksLikeURL(input string) bool {
	return strings.Contains(input, "kvartersmenyn.se/") ||
		strings.Contains(input, "http://") ||
		strings.Contains(input, "https://") ||
		strings.Contains(input, "index.php/") ||
		strings.Contains(input, "/area/") ||
		strings.Contains(input, "area/") ||
		strings.Contains(input, "/city/") ||
		strings.Contains(input, "city/")
}

func isNumericCity(city string) bool {
	if city == "" {
		return false
	}
	for _, r := range city {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
	if len(parts) < 1 {
		return "", "", false
	}

	var city string
	var area string
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "city":
			if i+1 < len(parts) {
				city = parts[i+1]
			}
		case "area":
			if i+1 < len(parts) {
				area = parts[i+1]
			}
		}
	}
	if city == "" {
		city = parts[0]
	}

	if city == "" {
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

	if dist, ok := safeRankMatchFold(normQuery, normName); ok {
		return dist >= 0 && dist <= maxDistance
	}
	return false
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
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func safeRankMatchFold(query, text string) (int, bool) {
	query = strings.ToValidUTF8(query, "")
	text = strings.ToValidUTF8(text, "")
	defer func() {
		if recover() != nil {
			// Fuzzy matcher can panic on unexpected input; treat as no match.
		}
	}()
	dist := fuzzy.RankMatchFold(query, text)
	return dist, true
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
	if dist, ok := safeRankMatchFold(normQuery, normText); ok {
		return dist >= 0 && dist <= maxDistance
	}
	return false
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

func parseDayFlag(input string) (int, bool) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return 0, false
	}
	switch input {
	case "1", "mon", "monday":
		return 1, true
	case "2", "tue", "tues", "tuesday":
		return 2, true
	case "3", "wed", "weds", "wednesday":
		return 3, true
	case "4", "thu", "thur", "thurs", "thursday":
		return 4, true
	case "5", "fri", "friday":
		return 5, true
	case "6", "sat", "saturday":
		return 6, true
	case "7", "sun", "sunday":
		return 7, true
	default:
		return 0, false
	}
}

func weekdayToDay(w time.Weekday) int {
	switch w {
	case time.Monday:
		return 1
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 3
	case time.Thursday:
		return 4
	case time.Friday:
		return 5
	case time.Saturday:
		return 6
	case time.Sunday:
		return 7
	default:
		return 1
	}
}

func dayLabel(day int) string {
	switch day {
	case 1:
		return "mon"
	case 2:
		return "tue"
	case 3:
		return "wed"
	case 4:
		return "thu"
	case 5:
		return "fri"
	case 6:
		return "sat"
	case 7:
		return "sun"
	default:
		return ""
	}
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
