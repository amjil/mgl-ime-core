package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockWordService struct {
	candidates     []string
	nextWords      []string
	candidateErr   error
	nextWordsErr   error
	receivedWord   string
	receivedForAPI string
}

func (m *mockWordService) GetCandidates(word string) ([]string, error) {
	m.receivedWord = word
	m.receivedForAPI = "candidates"
	return m.candidates, m.candidateErr
}

func (m *mockWordService) GetNextWords(word string) ([]string, error) {
	m.receivedWord = word
	m.receivedForAPI = "next"
	return m.nextWords, m.nextWordsErr
}

func TestHandleCandidatesSuccess(t *testing.T) {
	svc := &mockWordService{candidates: []string{"a", "b"}}
	handler := NewWordHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/next_word/candidates", bytes.NewBufferString(`{"word":"ab"}`))
	rec := httptest.NewRecorder()

	handler.HandleCandidates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.receivedWord != "ab" || svc.receivedForAPI != "candidates" {
		t.Fatalf("service not called as expected")
	}

	var resp map[string][]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp["candidates"]) != 2 {
		t.Fatalf("unexpected candidates length: %d", len(resp["candidates"]))
	}
}

func TestHandleCandidatesBadRequest(t *testing.T) {
	handler := NewWordHandler(&mockWordService{})
	req := httptest.NewRequest(http.MethodPost, "/api/next_word/candidates", bytes.NewBufferString(`{"word":"   "}`))
	rec := httptest.NewRecorder()

	handler.HandleCandidates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCandidatesInternalError(t *testing.T) {
	handler := NewWordHandler(&mockWordService{candidateErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodPost, "/api/next_word/candidates", bytes.NewBufferString(`{"word":"a"}`))
	rec := httptest.NewRecorder()

	handler.HandleCandidates(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleNextWordsSuccess(t *testing.T) {
	svc := &mockWordService{nextWords: []string{"x", "y"}}
	handler := NewWordHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/next_word/list", bytes.NewBufferString(`{"word":"ab"}`))
	rec := httptest.NewRecorder()

	handler.HandleNextWords(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.receivedWord != "ab" || svc.receivedForAPI != "next" {
		t.Fatalf("service not called as expected")
	}

	var resp map[string][]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp["nextWords"]) != 2 {
		t.Fatalf("unexpected nextWords length: %d", len(resp["nextWords"]))
	}
}
