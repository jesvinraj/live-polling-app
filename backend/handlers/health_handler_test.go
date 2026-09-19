package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHealthzEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHealthHandler(nil, nil)

	r := gin.New()
	r.GET("/healthz", handler.Healthz)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
}

func TestDetailedHealthCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHealthHandler(nil, nil)

	r := gin.New()
	r.GET("/api/v1/health", handler.DetailedHealth)

	// First call: Should perform check and not be cached
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w1.Code)
	}

	var resp1 map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("failed to decode response 1: %v", err)
	}

	if resp1["cached"] != false {
		t.Errorf("expected first call to have cached=false, got %v", resp1["cached"])
	}

	// Second immediate call: Should return cached response
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.ServeHTTP(w2, req2)

	var resp2 map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("failed to decode response 2: %v", err)
	}

	if resp2["cached"] != true {
		t.Errorf("expected second call within TTL to have cached=true, got %v", resp2["cached"])
	}

	// Test cache expiration by modifying cacheTTL
	handler.cacheTTL = 10 * time.Millisecond
	time.Sleep(20 * time.Millisecond)

	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.ServeHTTP(w3, req3)

	var resp3 map[string]interface{}
	if err := json.Unmarshal(w3.Body.Bytes(), &resp3); err != nil {
		t.Fatalf("failed to decode response 3: %v", err)
	}

	if resp3["cached"] != false {
		t.Errorf("expected call after TTL expiration to have cached=false, got %v", resp3["cached"])
	}
}
