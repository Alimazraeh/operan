package extract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Source describes the origin of content to extract.
type Source struct {
	URL      string
	Filename string
	MimeType string
}

// ExtractResult holds the full extracted content from a document.
type ExtractResult struct {
	Text      string
	Meta      map[string]string
	Sectioned []Section
}

// Section represents a heading-tagged portion of content.
type Section struct {
	Title   string
	Content string
	Page    int
}

// Extractor defines the interface for document content extraction.
type Extractor interface {
	Extract(ctx context.Context, source Source) (*ExtractResult, error)
	SupportedTypes() []string
}

// Downloader fetches content from a URL and returns bytes.
func Downloader(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("download: failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: HTTP %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB limit
	if err != nil {
		return nil, fmt.Errorf("download: read body: %w", err)
	}
	return data, nil
}

// DetectFileType returns the file extension from a filename or MIME type.
func DetectFileType(source Source) string {
	if source.MimeType != "" {
		switch {
		case strings.HasPrefix(source.MimeType, "application/pdf"):
			return "pdf"
		case strings.HasPrefix(source.MimeType, "application/vnd.openxmlformats"):
			return "docx"
		case strings.HasPrefix(source.MimeType, "application/vnd.ms-excel"):
			return "xlsx"
		case strings.HasPrefix(source.MimeType, "text/plain"):
			return "txt"
		case strings.HasPrefix(source.MimeType, "text/html"):
			return "html"
		}
	}
	if source.Filename != "" {
		lower := strings.ToLower(source.Filename)
		if strings.HasSuffix(lower, ".pdf") {
			return "pdf"
		}
		if strings.HasSuffix(lower, ".docx") {
			return "docx"
		}
		if strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls") {
			return "xlsx"
		}
		if strings.HasSuffix(lower, ".txt") {
			return "txt"
		}
		if strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") {
			return "html"
		}
	}
	return "unknown"
}

// NewExtractor returns the appropriate extractor for a given file type.
func NewExtractor(fileType string) Extractor {
	switch fileType {
	case "pdf":
		return NewPDFExtractor()
	case "docx":
		return NewDOCXExtractor()
	case "xlsx":
		return NewXLSXExtractor()
	case "txt":
		return NewTXTExtractor()
	case "html", "htm":
		return NewHTMLExtractor()
	default:
		return nil
	}
}

// Discover returns the supported file types.
func Discover() []string {
	return []string{"pdf", "docx", "xlsx", "txt", "html", "htm"}
}