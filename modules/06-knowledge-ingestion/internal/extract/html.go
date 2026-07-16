package extract

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTMLExtractor extracts text from HTML pages using goquery.
type HTMLExtractor struct{}

// NewHTMLExtractor creates a new HTMLExtractor.
func NewHTMLExtractor() *HTMLExtractor {
	return &HTMLExtractor{}
}

func (e *HTMLExtractor) SupportedTypes() []string {
	return []string{"html"}
}

func (e *HTMLExtractor) Extract(ctx context.Context, source Source) (*ExtractResult, error) {
	data, err := Downloader(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("html extract: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("html parse: %w", err)
	}

	// Remove nav, footer, and sidebar elements.
	doc.Find("nav, footer, aside, .sidebar, #sidebar, .nav, .menu").Remove()

	var sb strings.Builder
	var sections []Section
	sectionNum := 0

	// Extract headings and their content.
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		if sectionNum > 0 {
			sb.WriteString("\n\n")
		}
		sectionNum++
		title := strings.TrimSpace(s.Text())
		sb.WriteString(title)
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("=", len(title)))
		sb.WriteString("\n\n")

		next := s.NextUntil("h1, h2, h3, h4, h5, h6")
		next.Each(func(j int, ns *goquery.Selection) {
			text := strings.TrimSpace(ns.Text())
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		})
		sections = append(sections, Section{
			Title:   title,
			Content: strings.TrimSpace(next.Text()),
		})
	})

	// If no headings, extract all body text.
	if sectionNum == 0 {
		doc.Find("body p, body li, body td, body div").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		})
	}

	bodyText := strings.TrimSpace(sb.String())
	bodyText = strings.ReplaceAll(bodyText, "\n\n\n", "\n\n")

	meta := map[string]string{
		"file_type": "html",
		"sections":  fmt.Sprintf("%d", sectionNum),
		"title":     extractTitle(doc),
	}

	return &ExtractResult{
		Text:      bodyText,
		Meta:      meta,
		Sectioned: sections,
	}, nil
}

func extractTitle(doc *goquery.Document) string {
	return strings.TrimSpace(doc.Find("title").Text())
}