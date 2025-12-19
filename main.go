package main

import (
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
	"bytes"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

func main() {
	area := flag.String("area", "garda_161", "Area slug from kvartersmenyn, e.g. garda_161")
	city := flag.String("city", "goteborg", "City segment used in the kvartersmenyn URL")
	localFile := flag.String("file", "", "Optional local HTML file to parse instead of fetching from the site")
	search := flag.String("search", "", "Filter by restaurant name (fuzzy, case-insensitive)")
	cacheDir := flag.String("cache-dir", ".cache", "Directory for cached HTML (empty to disable)")
	cacheTTL := flag.Duration("cache-ttl", 6*time.Hour, "How long to reuse cached HTML")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var reader io.ReadCloser
	var sourceDesc string
	if *localFile != "" {
		file, err := os.Open(*localFile)
		if err != nil {
			log.Fatalf("kunde inte läsa filen %s: %v", *localFile, err)
		}
		reader = file
		sourceDesc = *localFile
	} else {
		if cache, desc, ok := tryCache(*cacheDir, *city, *area, *cacheTTL); ok {
			reader = cache
			sourceDesc = desc
		} else {
			url := buildAreaURL(*city, *area)
			resp, err := fetchHTML(ctx, url)
			if err != nil {
				log.Fatalf("kunde inte hämta data: %v", err)
			}
			reader, sourceDesc = cacheAndWrap(resp.Body, url, *cacheDir, *city, *area)
		}
	}
	defer reader.Close()

	restaurants, err := parseRestaurants(reader)
	if err != nil {
		log.Fatalf("kunde inte tolka sidan: %v", err)
	}

	query := strings.TrimSpace(*search)
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
