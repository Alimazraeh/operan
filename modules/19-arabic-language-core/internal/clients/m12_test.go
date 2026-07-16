package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedArabic_Success(t *testing.T) {
	// Mock M12 server
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models/embeddings" {
			t.Errorf("expected path /v1/models/embeddings, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify headers
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-jwt" {
			t.Errorf("expected Authorization 'Bearer test-jwt', got %q", auth)
		}
		if tenant := r.Header.Get("X-Tenant-ID"); tenant != "tenant-1" {
			t.Errorf("expected X-Tenant-ID 'tenant-1', got %q", tenant)
		}

		// Verify request body
		var req EmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "arabic-v1" {
			t.Errorf("expected model 'arabic-v1', got %q", req.Model)
		}
		if req.Input != "مرحبا بالعالم" {
			t.Errorf("unexpected input: %q", req.Input)
		}

		// Return mock response
		resp := EmbedResponse{
			Model:  "arabic-v1",
			Object: "list",
			Data: []EmbedItem{
				{Index: 0, Object: "embedding", Embedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	result, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا بالعالم", "test-jwt")
	if err != nil {
		t.Fatalf("EmbedArabic() error = %v", err)
	}

	if result.Model != "arabic-v1" {
		t.Errorf("expected model 'arabic-v1', got %q", result.Model)
	}

	if len(result.Data) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(result.Data))
	}

	if len(result.Data[0].Embedding) != 5 {
		t.Errorf("expected embedding dimension 5, got %d", len(result.Data[0].Embedding))
	}
}

func TestEmbedArabic_M12_500Error(t *testing.T) {
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestEmbedArabic_M12_404Error(t *testing.T) {
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestEmbedArabic_MissingBaseURL(t *testing.T) {
	client := NewM12Client("")

	ctx := context.Background()
	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err == nil {
		t.Fatal("expected error when base URL is empty, got nil")
	}

	if err.Error() != "M12_BASE_URL not configured" {
		t.Errorf("expected error 'M12_BASE_URL not configured', got %q", err.Error())
	}
}

func TestEmbedArabic_M12_Timeout(t *testing.T) {
	// Server that never responds
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang forever
		select {}
	}))
	defer m12Server.Close()

	// Create client with 100ms timeout
	client := &M12Client{
		baseURL: m12Server.URL,
		httpClient: &http.Client{
			Timeout: 100, // 100ms timeout
		},
	}

	ctx := context.Background()
	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
}

func TestEmbedArabic_EmptyResponse(t *testing.T) {
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := EmbedResponse{
			Model: "arabic-v1",
			Object: "list",
			Data:  []EmbedItem{}, // Empty data array
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	result, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err != nil {
		t.Fatalf("EmbedArabic() error = %v", err)
	}

	if len(result.Data) != 0 {
		t.Errorf("expected empty Data array, got %d items", len(result.Data))
	}
}

func TestEmbedArabic_MalformedResponse(t *testing.T) {
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err == nil {
		t.Fatal("expected error for malformed JSON response, got nil")
	}
}

func TestEmbedArabic_RequestHeaders(t *testing.T) {
	var receivedAuth, receivedTenant, receivedContentType string

	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedTenant = r.Header.Get("X-Tenant-ID")
		receivedContentType = r.Header.Get("Content-Type")

		resp := EmbedResponse{
			Model: "arabic-v1",
			Data:  []EmbedItem{{Index: 0, Embedding: []float64{0.1, 0.2}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", "مرحبا", "test-jwt")
	if err != nil {
		t.Fatalf("EmbedArabic() error = %v", err)
	}

	if receivedAuth != "Bearer test-jwt" {
		t.Errorf("expected Authorization 'Bearer test-jwt', got %q", receivedAuth)
	}
	if receivedTenant != "tenant-1" {
		t.Errorf("expected X-Tenant-ID 'tenant-1', got %q", receivedTenant)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", receivedContentType)
	}
}

func TestEmbedArabic_LargeText(t *testing.T) {
	m12Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := EmbedResponse{
			Model: "arabic-v1",
			Data:  []EmbedItem{{Index: 0, Embedding: []float64{0.1, 0.2, 0.3}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer m12Server.Close()

	client := NewM12Client(m12Server.URL)

	ctx := context.Background()
	// 10k+ character text
	largeText := "مرحبا بالعالم "
	for len(largeText) < 12000 {
		largeText += largeText
	}

	_, err := client.EmbedArabic(ctx, "tenant-1", "arabic-v1", largeText, "test-jwt")
	if err != nil {
		t.Fatalf("EmbedArabic() error for large text = %v", err)
	}
}

func TestM12Client_NewClient(t *testing.T) {
	client := NewM12Client("http://m12:8012")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://m12:8012" {
		t.Errorf("expected baseURL 'http://m12:8012', got %q", client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("expected non-nil httpClient")
	}
}