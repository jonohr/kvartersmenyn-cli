package main

import (
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type Restaurant struct {
	Name    string
	Price   string
	Address string
	Phone   string
	Link    string
	Menu    []string
}

// parseRestaurants scrapes the HTML into a list of restaurants.
func parseRestaurants(r io.Reader) ([]Restaurant, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var restaurants []Restaurant

	doc.Find("div.row.t_lunch").Each(func(_ int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("div.name h5.t_lunch a").First().Text())
		if name == "" {
			return
		}

		price := normalizeSpaces(s.Find(".price-rl .price").First().Text())
		menuLines := extractMenuLines(s.Find("div.rest-menu p.t_lunch").First())
		addrText := normalizeSpaces(s.Find(".divider p").First().Text())
		address, phone := splitAddressAndPhone(addrText)
		link, _ := s.Find("div.name h5.t_lunch a").First().Attr("href")

		restaurants = append(restaurants, Restaurant{
			Name:    name,
			Price:   price,
			Address: address,
			Phone:   phone,
			Link:    link,
			Menu:    menuLines,
		})
	})

	return restaurants, nil
}

func extractMenuLines(sel *goquery.Selection) []string {
	if sel.Length() == 0 {
		return nil
	}

	text := textWithBreaks(sel)
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = normalizeSpaces(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}

	return cleaned
}

// textWithBreaks keeps <br> as line breaks when extracting text.
func textWithBreaks(sel *goquery.Selection) string {
	var builder strings.Builder
	for _, node := range sel.Nodes {
		writeNode(&builder, node)
	}
	return strings.TrimSpace(builder.String())
}

func writeNode(builder *strings.Builder, node *html.Node) {
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "br" {
			builder.WriteString("\n")
			continue
		}
		writeNode(builder, child)
	}
}

func splitAddressAndPhone(line string) (string, string) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "ADRESS:"))
	var phone string

	if idx := strings.Index(strings.ToUpper(line), "TEL:"); idx >= 0 {
		rawPhone := line[idx+len("TEL:"):]
		phone = normalizeSpaces(rawPhone)
		line = strings.TrimSpace(line[:idx])
	}

	return line, phone
}

func normalizeSpaces(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.Join(strings.Fields(s), " ")
}
