package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestID_GeneratesUniqueID(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		if reqID == "" {
			t.Error("Expected RequestID in context")
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTraceID_PreservesAndGenerates(t *testing.T) {
	t.Run("preserves existing trace ID", func(t *testing.T) {
		h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceIDFromContext(r.Context())
			if traceID != "existing-trace-123" {
				t.Errorf("Expected existing-trace-123, got %s", traceID)
			}
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Trace-ID", "existing-trace-123")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	})

	t.Run("generates new trace ID", func(t *testing.T) {
		h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceIDFromContext(r.Context())
			if traceID == "" {
				t.Error("Expected generated trace ID")
			}
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	})
}

func TestTenantContext_RejectsMissingHeader(t *testing.T) {
	h := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 400 {
		t.Errorf("Expected error code 400, got %d", resp.Code)
	}
}

func TestTenantContext_ExtractsTenantID(t *testing.T) {
	h := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "tenant-abc-123" {
			t.Errorf("Expected tenant-abc-123, got %s", tenantID)
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	h := &Handler{}

	w := httptest.NewRecorder()
	data := map[string]interface{}{"key": "value", "count": 42}
	h.WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["key"] != "value" {
		t.Errorf("Expected value 'value', got %v", result["key"])
	}
}

func TestWriteError(t *testing.T) {
	h := &Handler{}

	w := httptest.NewRecorder()
	h.WriteError(w, http.StatusNotFound, 404, "not found", "resource does not exist")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 404 {
		t.Errorf("Expected error code 404, got %d", resp.Code)
	}
	if resp.Message != "not found" {
		t.Errorf("Expected message 'not found', got %s", resp.Message)
	}
	if resp.Details != "resource does not exist" {
		t.Errorf("Expected details 'resource does not exist', got %s", resp.Details)
	}
	if resp.RequestID == "" {
		t.Error("Expected non-empty RequestID")
	}
}

func TestMiddleware_Chaining(t *testing.T) {
	requestIDs := make([]string, 0, 3)

	h := RequestID(
		TraceID(
			TenantContext(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					reqID := TraceIDFromContext(r.Context())
					tenantID := TenantIDFromContext(r.Context())
					requestIDs = append(requestIDs, reqID)

					w.Header().Set("X-Request-ID", reqID)
					w.Header().Set("X-Tenant-ID", tenantID)
					w.WriteHeader(http.StatusOK)
				}),
			),
		),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if len(requestIDs) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requestIDs))
	}

	if requestIDs[0] == "" {
		t.Error("Expected non-empty request ID")
	}

	if w.Header().Get("X-Tenant-ID") != "tenant-123" {
		t.Errorf("Expected X-Tenant-ID 'tenant-123', got %s", w.Header().Get("X-Tenant-ID"))
	}
}

func TestPaginatedResponse_Marshaling(t *testing.T) {
	str1 := "draft"
	str2 := "active"
	resp := PaginatedResponse[string]{
		Data:    []*string{&str1, &str2},
		Total:   2,
		HasMore: false,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["total"].(float64) != 2 {
		t.Errorf("Expected total 2, got %v", result["total"])
	}
	if result["has_more"].(bool) != false {
		t.Errorf("Expected has_more false, got %v", result["has_more"])
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true
		if len(id) != 32 { // 16 bytes as hex
			t.Errorf("Expected ID length 32, got %d", len(id))
		}
	}
}

func TestLogger_LogsRequest(t *testing.T) {
	logged := false
	h := Logger(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logged = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/test/path", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("X-Request-ID", "req-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !logged {
		t.Error("Expected handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
