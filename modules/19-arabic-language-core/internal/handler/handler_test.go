package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleNormalize(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		wantCode int
	}{
		{
			name:     "empty body",
			body:     nil,
			wantCode: 400,
		},
		{
			name: "empty text",
			body: map[string]any{"text": ""},
			wantCode: 200, // Empty text is valid input, returns with dialect "unknown"
		},
		{
			name: "valid Arabic text",
			body: map[string]any{"text": "بسم الله الرحمن الرحيم"},
			wantCode: 200,
		},
		{
			name: "with options",
			body: map[string]any{
				"text":              "بِسْمِ اللَّهِ",
				"remove_tashkeel":   true,
				"normalize_alef":    true,
			},
			wantCode: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if tt.body != nil {
				json.NewEncoder(&body).Encode(tt.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/normalize", &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			HandleNormalize(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("HandleNormalize() code = %d, want %d; body: %s", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}

func TestHandleDetectDialect(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		wantCode int
	}{
		{
			name:     "empty body",
			body:     nil,
			wantCode: 200, // Nil body decodes to empty text, which is valid input
		},
		{
			name:     "empty text",
			body:     map[string]any{"text": ""},
			wantCode: 200, // Empty text is valid input, returns with dialect "unknown"
		},
		{
			name: "MSA text",
			body: map[string]any{"text": "إن هذا الكتاب يحتوي على معلومات مهمة"},
			wantCode: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			json.NewEncoder(&body).Encode(tt.body)

			req := httptest.NewRequest(http.MethodPost, "/v1/detect-dialect", &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			HandleDetectDialect(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("HandleDetectDialect() code = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestHandleStats(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	w := httptest.NewRecorder()

	HandleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HandleStats() code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["module"] != "arabic-language-core" {
		t.Errorf("HandleStats() module = %v, want 'arabic-language-core'", resp["module"])
	}
}

func TestWriteJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_ = req
	w := httptest.NewRecorder()

	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok"})

	if w.Code != http.StatusCreated {
		t.Errorf("writeJSON() code = %d, want %d", w.Code, http.StatusCreated)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("writeJSON() Content-Type = %q, want %q", ct, "application/json")
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("writeJSON() status = %v, want 'ok'", resp["status"])
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		haystack string
		needle   string
		want     bool
	}{
		{"مرحبا بالعالم", "مرحبا", true},
		{"مرحبا بالعالم", "عالم", true},
		{"مرحبا بالعالم", "سلام", false},
		{"مرحبا بالعالم", "", false},
		{"مرحبا بالعالم", "كلمة غير موجودة", false},
		{"مرحبا", "مرحبا بالعالم", false},
	}

	for _, tt := range tests {
		t.Run(tt.haystack+"_"+tt.needle, func(t *testing.T) {
			got := contains(tt.haystack, tt.needle)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.haystack, tt.needle, got, tt.want)
			}
		})
	}
}

func TestHandleNormalize_WithTashkeel(t *testing.T) {
	body := map[string]any{
		"text":              "بِسْمِ اللَّهِ",
		"remove_tashkeel":   true,
		"normalize_alef":    true,
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/normalize", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleNormalize(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HandleNormalize() code = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp NormalizeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.NormalizedLength >= resp.OriginalLength {
		t.Errorf("expected normalized length < original, got orig=%d, norm=%d",
			resp.OriginalLength, resp.NormalizedLength)
	}

	hasAction := false
	for _, a := range resp.Actions {
		if a.Type == "remove_tashkeel" {
			hasAction = true
			break
		}
	}
	if !hasAction {
		t.Error("expected at least one remove_tashkeel action")
	}
}

func TestHandleNormalize_MixedLanguage(t *testing.T) {
	body := map[string]any{
		"text": "مرحبا world hello 123",
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/normalize", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleNormalize(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HandleNormalize() code = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp NormalizeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Normalized) == 0 {
		t.Error("expected non-empty normalized text")
	}
}

func TestHandleDetectDialect_ArabicText(t *testing.T) {
	body := map[string]any{
		"text": "إن شاء الله هذا الكتاب رائع جداً",
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/detect-dialect", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleDetectDialect(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HandleDetectDialect() code = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp DetectDialectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Confidence < 0 || resp.Confidence > 1 {
		t.Errorf("expected confidence in [0, 1], got %f", resp.Confidence)
	}

	if len(resp.AllScores) == 0 {
		t.Error("expected non-empty all_scores")
	}
}

func TestHandleStats_JSONContent(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	w := httptest.NewRecorder()

	HandleStats(w, httptest.NewRequest(http.MethodGet, "/v1/stats", nil))

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", w.Header().Get("Content-Type"))
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := resp["module"]; !ok {
		t.Error("expected 'module' field in response")
	}
	if _, ok := resp["version"]; !ok {
		t.Error("expected 'version' field in response")
	}
}

func TestHandleNormalize_MalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/normalize", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleNormalize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed JSON, got %d", w.Code)
	}
}