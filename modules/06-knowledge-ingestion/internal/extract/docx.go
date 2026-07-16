package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DOCXExtractor extracts text from DOCX files using archive/zip and encoding/xml.
type DOCXExtractor struct{}

// NewDOCXExtractor creates a new DOCXExtractor.
func NewDOCXExtractor() *DOCXExtractor {
	return &DOCXExtractor{}
}

func (e *DOCXExtractor) SupportedTypes() []string {
	return []string{"docx"}
}

func (e *DOCXExtractor) Extract(ctx context.Context, source Source) (*ExtractResult, error) {
	data, err := Downloader(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("docx extract: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docx open zip: %w", err)
	}

	var paragraphs []string
	var sections []Section
	currentHeading := ""
	var headingContent strings.Builder

	for _, f := range reader.File {
		if f.Name != "word/document.xml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		defer rc.Close()

		xmlData, err := io.ReadAll(rc)
		if err != nil {
			continue
		}

		type wP struct {
			Text string `xml:"w:t>chardata"`
		}
		type wBody struct {
			Ps []wP `xml:"w:p"`
		}
		var body wBody
		if err := xml.Unmarshal(xmlData, &body); err != nil {
			continue
		}

		paraNum := 0
		for _, p := range body.Ps {
			text := strings.TrimSpace(p.Text)
			if text == "" {
				continue
			}
			paraNum++

			if isHeadingText(text) {
				if currentHeading != "" && headingContent.Len() > 0 {
					sections = append(sections, Section{
						Title:   currentHeading,
						Content: strings.TrimSpace(headingContent.String()),
					})
				}
				currentHeading = text
				headingContent.Reset()
			} else {
				if currentHeading == "" {
					sections = append(sections, Section{
						Title:   fmt.Sprintf("Paragraph %d", paraNum),
						Content: text,
					})
				} else {
					if headingContent.Len() > 0 {
						headingContent.WriteString("\n")
					}
					headingContent.WriteString(text)
				}
			}
			paragraphs = append(paragraphs, text)
		}

		if currentHeading != "" && headingContent.Len() > 0 {
			sections = append(sections, Section{
				Title:   currentHeading,
				Content: strings.TrimSpace(headingContent.String()),
			})
		}

		break
	}

	meta := map[string]string{
		"file_type":       "docx",
		"paragraph_count": fmt.Sprintf("%d", len(paragraphs)),
		"title":           source.Filename,
	}

	return &ExtractResult{
		Text:      strings.Join(paragraphs, "\n"),
		Meta:      meta,
		Sectioned: sections,
	}, nil
}

func isHeadingText(text string) bool {
	if len(text) > 0 && text == strings.ToUpper(text) && len(text) < 100 {
		return true
	}
	fields := strings.Fields(text)
	if len(fields) <= 6 && len(text) < 80 {
		if strings.Contains(text, "1.") || strings.Contains(text, "Chapter") ||
			strings.Contains(text, "Section") || strings.Contains(text, "Part") {
			return true
		}
	}
	return false
}