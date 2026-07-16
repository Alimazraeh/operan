package extract

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// XLSXExtractor extracts text from XLSX files using excelize.
type XLSXExtractor struct{}

// NewXLSXExtractor creates a new XLSXExtractor.
func NewXLSXExtractor() *XLSXExtractor {
	return &XLSXExtractor{}
}

func (e *XLSXExtractor) SupportedTypes() []string {
	return []string{"xlsx"}
}

func (e *XLSXExtractor) Extract(ctx context.Context, source Source) (*ExtractResult, error) {
	data, err := Downloader(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("xlsx extract: %w", err)
	}

	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("xlsx open: %w", err)
	}
	defer file.Close()

	sheetMap := file.GetSheetMap()
	var sheetNames []string
	for _, name := range sheetMap {
		sheetNames = append(sheetNames, name)
	}

	var sb strings.Builder
	meta := map[string]string{
		"file_type":   "xlsx",
		"sheet_count": fmt.Sprintf("%d", len(sheetNames)),
		"title":       source.Filename,
	}

	for _, sheetName := range sheetNames {
		sb.WriteString(fmt.Sprintf("=== Sheet: %s ===\n\n", sheetName))

		rows, err := file.Rows(sheetName)
		if err != nil {
			sb.WriteString(fmt.Sprintf("  Error reading sheet %s: %v\n", sheetName, err))
			continue
		}

		rowNum := 0
		for rows.Next() {
			cells, err := rows.Columns()
			if err != nil {
				break
			}
			rowNum++
			var parts []string
			for _, cell := range cells {
				parts = append(parts, cell)
			}
			sb.WriteString(strings.Join(parts, "\t"))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return &ExtractResult{
		Text: strings.TrimSpace(sb.String()),
		Meta: meta,
	}, nil
}