package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthReturnsOK(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	NewRouter(nil).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestUnknownRouteReturnsNotFound(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter(nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}
