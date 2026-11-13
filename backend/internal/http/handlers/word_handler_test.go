package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mgl-ime-core/backend/internal/word/store"
)

type mockWordService struct {
	candidates     []store.ScoredWord
	nextWords      []store.ScoredWord
	candidateErr   error
	nextWordsErr   error
	addUserWordErr error
	receivedWord   string
	receivedNext   []string
	receivedForAPI string
}

func (m *mockWordService) GetCandidates(word string) ([]store.ScoredWord, error) {
	m.receivedWord = word
	m.receivedForAPI = "candidates"
	return m.candidates, m.candidateErr
}

func (m *mockWordService) GetNextWords(word string) ([]store.ScoredWord, error) {
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
	svc := &mockWordService{candidates: []store.ScoredWord{
		{Word: "a", Order: store.UserFeedbackBaseOrder},
		{Word: "b", Order: 2},
	}}
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

	var resp map[string][][]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	got := resp["candidates"]
	if len(got) != 2 || got[0][0] != "a" || got[1][0] != "b" {
		t.Fatalf("unexpected candidates: %#v", got)
	}
	order, ok := got[0][1].(float64)
	if !ok || order < store.UserFeedbackBaseOrder {
		t.Fatalf("feedback order = %#v", got[0][1])
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
	svc := &mockWordService{nextWords: []store.ScoredWord{
		{Word: "x", Order: store.UserFeedbackBaseOrder},
		{Word: "y", Order: 1},
	}}
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

	var resp map[string][][]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	got := resp["nextWords"]
	if len(got) != 2 || got[0][0] != "x" || got[1][0] != "y" {
		t.Fatalf("unexpected nextWords: %#v", got)
	}
	order, ok := got[0][1].(float64)
	if !ok || order < store.UserFeedbackBaseOrder || order <= got[1][1].(float64) {
		t.Fatalf("feedback next word not ranked first: %#v", got)
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

type mockScoredWordService struct {
	mockWordService
	receivedScoredWord store.ScoredWord
	receivedScoredNext []store.ScoredWord
}

func (m *mockScoredWordService) AddUserScoredWord(word store.ScoredWord, nextWords []store.ScoredWord) error {
	m.receivedScoredWord = word
	m.receivedScoredNext = nextWords
	m.receivedForAPI = "add-user-scored-word"
	return nil
}

func TestHandleAddUserWordWithWeightedTuples(t *testing.T) {
	svc := &mockScoredWordService{}
	handler := NewWordHandler(svc)

	body := `{"word":"ᠮᠤᠩᠭᠤᠯ","nextWords":[["ᠤᠯᠤᠰ",10],["ᠬᠡᠯᠡ",50],"plain"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/user_words", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.HandleAddUserWord(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.receivedForAPI != "add-user-scored-word" {
		t.Fatalf("expected scored path, got %q", svc.receivedForAPI)
	}
	if svc.receivedScoredWord.Word != "ᠮᠤᠩᠭᠤᠯ" {
		t.Fatalf("word = %#v", svc.receivedScoredWord)
	}
	if len(svc.receivedScoredNext) != 3 {
		t.Fatalf("next = %#v", svc.receivedScoredNext)
	}
	if svc.receivedScoredNext[0].Word != "ᠤᠯᠤᠰ" || svc.receivedScoredNext[0].Order != 10 {
		t.Fatalf("first = %#v", svc.receivedScoredNext[0])
	}
	if svc.receivedScoredNext[1].Word != "ᠬᠡᠯᠡ" || svc.receivedScoredNext[1].Order != 50 {
		t.Fatalf("second = %#v", svc.receivedScoredNext[1])
	}
	if svc.receivedScoredNext[2].Word != "plain" || svc.receivedScoredNext[2].Order != 0 {
		t.Fatalf("third = %#v", svc.receivedScoredNext[2])
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
