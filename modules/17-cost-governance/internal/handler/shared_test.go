package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]int{"total": 42}
	WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var result map[string]int
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["total"] != 42 {
		t.Errorf("expected total 42, got %d", result["total"])
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusCreated, nil)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestWriteJSON_DifferentStatusCodes(t *testing.T) {
	tests := []int{200, 201, 400, 404, 500}
	for _, code := range tests {
		t.Run(string(rune(code)), func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJSON(w, code, map[string]string{"ok": "true"})
			if w.Code != code {
				t.Errorf("expected %d, got %d", code, w.Code)
			}
		})
	}
}

func TestPeriodStart(t *testing.T) {
	// Monday of the current week
	jan4 := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC) // Sunday
	result := periodStart("weekly", jan4)
	// Should go back to Monday (Dec 29, 2025)
	expected := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("weekly from Sunday Jan 4: expected %v, got %v", expected, result)
	}

	// Wednesday of current week
	wedJan7 := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC) // Wednesday
	result = periodStart("weekly", wedJan7)
	expected = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("weekly from Wednesday Jan 7: expected %v, got %v", expected, result)
	}

	// Daily
	daily := time.Date(2026, 3, 15, 23, 59, 59, 0, time.UTC)
	result = periodStart("daily", daily)
	expected = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("daily: expected %v, got %v", expected, result)
	}

	// Monthly
	monthly := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	result = periodStart("monthly", monthly)
	expected = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("monthly: expected %v, got %v", expected, result)
	}

	// Quarterly Q3
	q3 := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	result = periodStart("quarterly", q3)
	expected = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("quarterly Q3: expected %v, got %v", expected, result)
	}

	// Quarterly Q1
	q1 := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	result = periodStart("quarterly", q1)
	expected = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("quarterly Q1: expected %v, got %v", expected, result)
	}
}

func TestJSONEncode(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 200, struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}{"Alice", 30})

	body := w.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"name":"Alice"`)) {
		t.Errorf("expected Alice in response, got %s", body)
	}
	if !bytes.Contains([]byte(body), []byte(`"age":30`)) {
		t.Errorf("expected 30 in response, got %s", body)
	}
}