package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"mgl-ime-core/backend/internal/word/store"
)

const (
	maxRequestBodyBytes   = 4096
	maxWordRunes          = 256
	maxNextWordsInRequest = 64
)

type WordService interface {
	GetCandidates(word string) ([]store.ScoredWord, error)
	GetNextWords(word string) ([]store.ScoredWord, error)
	AddUserWord(word string, nextWords []string) error
}

type WordHandler struct {
	service WordService
}

func NewWordHandler(service WordService) *WordHandler {
	return &WordHandler{service: service}
}

type wordRequest struct {
	Word string `json:"word"`
}

type addUserWordRequest struct {
	Word      string   `json:"word"`
	NextWords []string `json:"nextWords"`
}

// scoredTuple serializes directly as a JSON 2-tuple ["word", order] without []any boxing.
type scoredTuple store.ScoredWord

func (t scoredTuple) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(len(t.Word) + 16)
	buf.WriteByte('[')
	wordJSON, err := json.Marshal(t.Word)
	if err != nil {
		return nil, err
	}
	buf.Write(wordJSON)
	buf.WriteByte(',')
	buf.WriteString(strconv.FormatFloat(t.Order, 'f', -1, 64))
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

type candidatesResponse struct {
	Candidates []scoredTuple `json:"candidates"`
}

type nextWordsResponse struct {
	NextWords []scoredTuple `json:"nextWords"`
}

func (h *WordHandler) HandleCandidates(w http.ResponseWriter, r *http.Request) {
	word, err := parseWordRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	candidates, err := h.service.GetCandidates(word)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query candidates")
		return
	}

	writeJSON(w, http.StatusOK, candidatesResponse{Candidates: encodeScoredList(candidates)})
}

func (h *WordHandler) HandleAddUserWord(w http.ResponseWriter, r *http.Request) {
	word, nextWords, err := parseAddUserWordRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.AddUserWord(word, nextWords); err != nil {
		if errors.Is(err, store.ErrUserDictNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save user word")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *WordHandler) HandleNextWords(w http.ResponseWriter, r *http.Request) {
	word, err := parseWordRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	nextWords, err := h.service.GetNextWords(word)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query next words")
		return
	}

	writeJSON(w, http.StatusOK, nextWordsResponse{NextWords: encodeScoredList(nextWords)})
}

func encodeScoredList(words []store.ScoredWord) []scoredTuple {
	out := make([]scoredTuple, 0, len(words))
	for _, w := range words {
		if w.Word == "" {
			continue
		}
		out = append(out, scoredTuple(w))
	}
	return out
}

func parseAddUserWordRequest(r *http.Request) (word string, nextWords []string, err error) {
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodyBytes)

	var req addUserWordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return "", nil, errors.New("request body too large")
		}
		return "", nil, errors.New("invalid json body")
	}

	word = strings.TrimSpace(req.Word)
	if word == "" {
		return "", nil, errors.New("word is required")
	}
	if utf8.RuneCountInString(word) > maxWordRunes {
		return "", nil, errors.New("word is too long")
	}

	if len(req.NextWords) > maxNextWordsInRequest {
		return "", nil, errors.New("nextWords is too long")
	}

	nextWords = make([]string, 0, len(req.NextWords))
	for _, raw := range req.NextWords {
		w := strings.TrimSpace(raw)
		if w == "" {
			continue
		}
		if utf8.RuneCountInString(w) > maxWordRunes {
			return "", nil, errors.New("nextWords entry is too long")
		}
		nextWords = append(nextWords, w)
	}
	return word, nextWords, nil
}

func parseWordRequest(r *http.Request) (string, error) {
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodyBytes)

	var req wordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return "", errors.New("request body too large")
		}
		return "", errors.New("invalid json body")
	}

	word := strings.TrimSpace(req.Word)
	if word == "" {
		return "", errors.New("word is required")
	}
	if utf8.RuneCountInString(word) > maxWordRunes {
		return "", errors.New("word is too long")
	}
	return word, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
