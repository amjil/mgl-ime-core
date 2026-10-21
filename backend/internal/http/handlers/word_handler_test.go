package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockWordService struct {
	candidates     []string
	nextWords      []string
	candidateErr   error
	nextWordsErr   error
	addUserWordErr error
	receivedWord   string
	receivedNext   []string
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

func (m *mockWordService) AddUserWord(word string, nextWords []string) error {
	m.receivedWord = word
	m.receivedNext = nextWords
	m.receivedForAPI = "add-user-word"
	return m.addUserWordErr
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

func TestHandleCandidatesInvalidJSON(t *testing.T) {
	handler := NewWordHandler(&mockWordService{})
	req := httptest.NewRequest(http.MethodPost, "/api/next_word/candidates", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	handler.HandleCandidates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCandidatesWordTooLong(t *testing.T) {
	handler := NewWordHandler(&mockWordService{})
	word := make([]byte, 0, maxWordRunes+1)
	for i := 0; i < maxWordRunes+1; i++ {
		word = append(word, 'a')
	}
	req := httptest.NewRequest(http.MethodPost, "/api/next_word/candidates", bytes.NewBufferString(`{"word":"`+string(word)+`"}`))
	rec := httptest.NewRecorder()

	handler.HandleCandidates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleAddUserWordSuccess(t *testing.T) {
	svc := &mockWordService{}
	handler := NewWordHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/user_words", bytes.NewBufferString(`{"word":"ᠠ","nextWords":["ᠰᠡᠪᠡ","ᠬᠡ"]}`))
	rec := httptest.NewRecorder()

	handler.HandleAddUserWord(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.receivedWord != "ᠠ" || svc.receivedForAPI != "add-user-word" {
		t.Fatalf("unexpected service call: word=%q api=%q", svc.receivedWord, svc.receivedForAPI)
	}
	if len(svc.receivedNext) != 2 || svc.receivedNext[0] != "ᠰᠡᠪᠡ" || svc.receivedNext[1] != "ᠬᠡ" {
		t.Fatalf("unexpected nextWords: %#v", svc.receivedNext)
	}
}

func TestHandleCandidatesBodyTooLarge(t *testing.T) {
	handler := NewWordHandler(&mockWordService{})
	body := `{"word":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/next_word/candidates", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.HandleCandidates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp["error"] != "request body too large" {
		t.Fatalf("unexpected error: %#v", resp)
	}
}
