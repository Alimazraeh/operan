package extract

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// PDFExtractor extracts text from PDF files using simple content stream parsing.
type PDFExtractor struct{}

// NewPDFExtractor creates a new PDFExtractor.
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

func (e *PDFExtractor) SupportedTypes() []string {
	return []string{"pdf"}
}

func (e *PDFExtractor) Extract(ctx context.Context, source Source) (*ExtractResult, error) {
	data, err := Downloader(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("pdf extract: %w", err)
	}

	text, pageCount, sections, err := extractPDFText(data)
	if err != nil {
		return nil, fmt.Errorf("pdf extract: %w", err)
	}

	meta := map[string]string{
		"file_type":  "pdf",
		"page_count": fmt.Sprintf("%d", pageCount),
		"title":      source.Filename,
	}

	if strings.TrimSpace(text) == "" {
		meta["warning"] = "no text layer found in PDF (may be scanned image)"
	}

	return &ExtractResult{
		Text:      text,
		Meta:      meta,
		Sectioned: sections,
	}, nil
}

var textOpPattern = regexp.MustCompile(`\(([^)]*)\)Tj|\(([^)]*)\)'\(([^)]*)\)"` )

func extractPDFText(data []byte) (string, int, []Section, error) {
	content := string(data)

	// Count pages by looking for /Type /Page entries.
	pagePattern := regexp.MustCompile(`/Type\s*/Page[^s]`)
	pageCount := len(pagePattern.FindAllString(content, -1))
	if pageCount == 0 {
		pageCount = 1
	}

	var sb strings.Builder
	var sections []Section

	// Extract text between Tj, ', " operators.
	textStrings := textOpPattern.FindAllStringSubmatch(content, -1)
	for _, match := range textStrings {
		text := ""
		for _, s := range match[1:] {
			if s != "" {
				text = s
				break
			}
		}
		if text == "" {
			continue
		}
		text = unescapePDFString(text)
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	// Also try to extract text from BT ... ET blocks.
	btPattern := regexp.MustCompile(`BT\s(.*?)\sET`)
	blocks := btPattern.FindAllStringSubmatch(content, -1)
	for _, block := range blocks {
		if block[1] != "" {
			innerText := textOpPattern.FindAllStringSubmatch(block[1], -1)
			for _, match := range innerText {
				text := ""
				for _, s := range match[1:] {
					if s != "" {
						text = s
						break
					}
				}
				if text != "" {
					sections = append(sections, Section{
						Content: text,
					})
				}
			}
		}
	}

	return strings.TrimSpace(sb.String()), pageCount, sections, nil
}

// unescapePDFString unescapes PDF string escape sequences.
func unescapePDFString(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	s = strings.ReplaceAll(s, "\\(", "(")
	s = strings.ReplaceAll(s, "\\)", ")")
	return s
}