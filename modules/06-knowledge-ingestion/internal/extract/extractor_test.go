package extract

import (
	"context"
	"testing"
)

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		expected string
	}{
		{"PDF by extension", Source{Filename: "doc.pdf"}, "pdf"},
		{"PDF by MIME", Source{MimeType: "application/pdf"}, "pdf"},
		{"DOCX by extension", Source{Filename: "report.docx"}, "docx"},
		{"DOCX by MIME", Source{MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}, "docx"},
		{"XLSX by extension", Source{Filename: "data.xlsx"}, "xlsx"},
		{"XLSX by MIME", Source{MimeType: "application/vnd.ms-excel"}, "xlsx"},
		{"TXT by extension", Source{Filename: "notes.txt"}, "txt"},
		{"TXT by MIME", Source{MimeType: "text/plain"}, "txt"},
		{"HTML by extension", Source{Filename: "page.html"}, "html"},
		{"HTML by MIME", Source{MimeType: "text/html"}, "html"},
		{"Unknown", Source{Filename: "image.png"}, "unknown"},
		{"Empty", Source{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFileType(tt.source)
			if got != tt.expected {
				t.Errorf("DetectFileType(%v) = %q, want %q", tt.source, got, tt.expected)
			}
		})
	}
}

// === PDF Extractor Tests ===

func TestPDFExtractor_SupportedTypes(t *testing.T) {
	e := NewPDFExtractor()
	types := e.SupportedTypes()
	if len(types) != 1 || types[0] != "pdf" {
		t.Errorf("expected [pdf], got %v", types)
	}
}

func TestPDFExtractor_InvalidURL(t *testing.T) {
	e := NewPDFExtractor()
	ctx := context.Background()
	source := Source{URL: "http://invalid-url-that-does-not-exist.example.com/file.pdf"}
	_, err := e.Extract(ctx, source)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// === DOCX Extractor Tests ===

func TestDOCXExtractor_SupportedTypes(t *testing.T) {
	e := NewDOCXExtractor()
	types := e.SupportedTypes()
	if len(types) != 1 || types[0] != "docx" {
		t.Errorf("expected [docx], got %v", types)
	}
}

func TestDOCXExtractor_InvalidURL(t *testing.T) {
	e := NewDOCXExtractor()
	ctx := context.Background()
	source := Source{URL: "http://invalid-url-that-does-not-exist.example.com/file.docx"}
	_, err := e.Extract(ctx, source)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// === XLSX Extractor Tests ===

func TestXLSXExtractor_SupportedTypes(t *testing.T) {
	e := NewXLSXExtractor()
	types := e.SupportedTypes()
	if len(types) != 1 || types[0] != "xlsx" {
		t.Errorf("expected [xlsx], got %v", types)
	}
}

func TestXLSXExtractor_InvalidURL(t *testing.T) {
	e := NewXLSXExtractor()
	ctx := context.Background()
	source := Source{URL: "http://invalid-url-that-does-not-exist.example.com/file.xlsx"}
	_, err := e.Extract(ctx, source)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// === TXT Extractor Tests ===

func TestTXTExtractor_SupportedTypes(t *testing.T) {
	e := NewTXTExtractor()
	types := e.SupportedTypes()
	if len(types) != 1 || types[0] != "txt" {
		t.Errorf("expected [txt], got %v", types)
	}
}

func TestTXTExtractor_InvalidURL(t *testing.T) {
	e := NewTXTExtractor()
	ctx := context.Background()
	source := Source{URL: "http://invalid-url-that-does-not-exist.example.com/file.txt"}
	_, err := e.Extract(ctx, source)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// === HTML Extractor Tests ===

func TestHTMLExtractor_SupportedTypes(t *testing.T) {
	e := NewHTMLExtractor()
	types := e.SupportedTypes()
	if len(types) != 1 || types[0] != "html" {
		t.Errorf("expected [html], got %v", types)
	}
}

func TestHTMLExtractor_InvalidURL(t *testing.T) {
	e := NewHTMLExtractor()
	ctx := context.Background()
	source := Source{URL: "http://invalid-url-that-does-not-exist.example.com/file.html"}
	_, err := e.Extract(ctx, source)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// === NewExtractor Tests ===

func TestNewExtractor_ReturnsPDF(t *testing.T) {
	e := NewExtractor("pdf")
	types := e.SupportedTypes()
	found := false
	for _, t2 := range types {
		if t2 == "pdf" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NewExtractor to support pdf, got %v", types)
	}
}