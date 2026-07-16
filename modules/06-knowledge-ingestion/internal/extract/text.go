package extract

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// TextExtractor extracts text from plain text files.
type TextExtractor struct{}

// NewTXTExtractor creates a new TextExtractor.
func NewTXTExtractor() *TextExtractor {
	return &TextExtractor{}
}

func (e *TextExtractor) SupportedTypes() []string {
	return []string{"txt"}
}

func (e *TextExtractor) Extract(ctx context.Context, source Source) (*ExtractResult, error) {
	data, err := Downloader(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("text extract: %w", err)
	}

	text := string(data)
	if len(text) > 10*1024*1024 {
		text = text[:10*1024*1024]
	}

	trimmed := strings.TrimSpace(text)
	sections := []Section{
		{
			Title:   source.Filename,
			Content: trimmed,
			Page:    1,
		},
	}

	meta := map[string]string{
		"file_type": "txt",
		"size":      fmt.Sprintf("%d", len(data)),
		"title":     source.Filename,
	}

	return &ExtractResult{
		Text:      trimmed,
		Meta:      meta,
		Sectioned: sections,
	}, nil
}

// ExtractFromFile is a helper for extracting text from a local file path.
func ExtractFromFile(path string) (string, error) {
	f, err := openFile(path)
	if err != nil {
		return "", err
	}
	defer closeReader(f)

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// openFile opens a file reader.
func openFile(path string) (io.Reader, error) {
	return nil, fmt.Errorf("not implemented: use Downloader instead")
}

// closeReader closes a reader.
func closeReader(r io.Reader) {}