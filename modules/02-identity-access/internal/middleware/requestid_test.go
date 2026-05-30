package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestRequestIDMiddleware_GeneratesNewID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("GetRequestID() returned empty string")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequestIDMiddleware() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRequestIDMiddleware_PreservesExistingID(t *testing.T) {
	expectedID := "existing-request-id-123"

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID != expectedID {
			t.Errorf("GetRequestID() = %v, want %v", requestID, expectedID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", expectedID)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequestIDMiddleware() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRequestIDMiddleware_InResponseHeaders(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("X-Request-ID header not set in response")
	}
}

func TestRequestIDMiddleware_ResponseMatchesContext(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		responseID := w.Header().Get("X-Request-ID")
		if requestID != responseID {
			t.Errorf("context ID (%v) != response header ID (%v)", requestID, responseID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequestIDMiddleware() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRequestIDMiddleware_ExistingIDPropagatedToResponse(t *testing.T) {
	expectedID := "client-provided-id-456"

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", expectedID)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-ID")
	if responseID != expectedID {
		t.Errorf("X-Request-ID header = %v, want %v", responseID, expectedID)
	}
}

func TestRequestIDMiddleware_UUIDFormat(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Fatal("X-Request-ID header is empty")
	}

	// Validate it parses as a valid UUID v4
	_, err := uuid.Parse(responseID)
	if err != nil {
		t.Errorf("X-Request-ID (%v) is not a valid UUID: %v", responseID, err)
	}
}

func TestRequestIDMiddleware_GeneratedIDIsValidUUID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Run multiple times to confirm generated IDs are valid UUIDs
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		responseID := w.Header().Get("X-Request-ID")
		_, err := uuid.Parse(responseID)
		if err != nil {
			t.Errorf("iteration %d: X-Request-ID (%v) is not a valid UUID: %v", i, responseID, err)
		}
	}
}

func TestRequestIDMiddleware_UniqueIDs(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ids := make(map[string]bool, 20)
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		responseID := w.Header().Get("X-Request-ID")
		if ids[responseID] {
			t.Errorf("duplicate request ID generated: %v", responseID)
		}
		ids[responseID] = true
	}
}

func TestGetRequestID_FromContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := SetRequestID(req.Context(), "test-request-id")
	req = req.WithContext(ctx)

	got := GetRequestID(req.Context())
	if got != "test-request-id" {
		t.Errorf("GetRequestID() = %v, want test-request-id", got)
	}
}

func TestGetRequestID_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := GetRequestID(req.Context())
	if got != "" {
		t.Errorf("GetRequestID() = %v, want empty string", got)
	}
}

func TestRequestIDMiddleware_ConcurrentAccess(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("GetRequestID() returned empty string in goroutine")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// Use a wait group to ensure all goroutines complete before checking results
	var wg sync.WaitGroup
	errors := make(chan string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			responseID := w.Header().Get("X-Request-ID")
			if responseID == "" {
				errors <- "empty X-Request-ID in response"
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestRequestIDMiddleware_ChainedWithTraceInjector(t *testing.T) {
	// Ensure RequestIDMiddleware can be chained with TraceInjector (existing middleware)
	handler := RequestIDMiddleware(TraceInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		traceID := GetTraceID(r.Context())
		if requestID == "" {
			t.Error("GetRequestID() returned empty string in chain")
		}
		if traceID == "" {
			t.Error("GetTraceID() returned empty string in chain")
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("chained middleware status = %v, want %v", w.Code, http.StatusOK)
	}

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header not set in chained middleware")
	}

	if w.Header().Get("X-Trace-ID") == "" {
		t.Error("X-Trace-ID header not set in chained middleware")
	}
}

func TestRequestIDMiddleware_RequestIDHasHyphens(t *testing.T) {
	// UUID v4 format contains hyphens: 8-4-4-4-12
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Fatal("X-Request-ID header is empty")
	}

	// UUID v4 format: xxxxxxxx-xxxx-4xxx-[89ab]xxx-xxxxxxxxxxxx
	// Has 4 hyphens
	hyphenCount := strings.Count(responseID, "-")
	if hyphenCount != 4 {
		t.Errorf("X-Request-ID (%v) expected 4 hyphens for UUID format, got %d", responseID, hyphenCount)
	}

	if len(responseID) != 36 {
		t.Errorf("X-Request-ID length = %d, want 36 (UUID v4 format)", len(responseID))
	}
}
