package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubWordHandler struct{}

func (stubWordHandler) HandleCandidates(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (stubWordHandler) HandleNextWords(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (stubWordHandler) HandleAddUserWord(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestHealthz(t *testing.T) {
	router := NewRouter(stubWordHandler{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	router := NewRouter(stubWordHandler{})
	req := httptest.NewRequest(http.MethodOptions, "/api/next_word/candidates", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS origin header")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	router := NewRouter(stubWordHandler{})
	req := httptest.NewRequest(http.MethodGet, "/api/next_word/candidates", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
