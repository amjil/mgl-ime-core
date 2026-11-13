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

type ScoredWordService interface {
	WordService
	AddUserScoredWord(word store.ScoredWord, nextWords []store.ScoredWord) error
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
	Word      string            `json:"word"`
	Order     float64           `json:"order"`
	NextWords []json.RawMessage `json:"nextWords"`
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

	var saveErr error
	hasCustomOrder := word.Order != 0
	for _, nw := range nextWords {
		if nw.Order != 0 {
			hasCustomOrder = true
			break
		}
	}

	if scoredSvc, ok := h.service.(ScoredWordService); ok && hasCustomOrder {
		saveErr = scoredSvc.AddUserScoredWord(word, nextWords)
	} else {
		plainNext := make([]string, 0, len(nextWords))
		for _, nw := range nextWords {
			plainNext = append(plainNext, nw.Word)
		}
		saveErr = h.service.AddUserWord(word.Word, plainNext)
	}

	if saveErr != nil {
		if errors.Is(saveErr, store.ErrUserDictNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, saveErr.Error())
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

func parseAddUserWordRequest(r *http.Request) (word store.ScoredWord, nextWords []store.ScoredWord, err error) {
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodyBytes)
	var req addUserWordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return store.ScoredWord{}, nil, errors.New("request body too large")
		}
		return store.ScoredWord{}, nil, errors.New("invalid json body")
	}

	trimmed := strings.TrimSpace(req.Word)
	if trimmed == "" {
		return store.ScoredWord{}, nil, errors.New("word is required")
	}
	if utf8.RuneCountInString(trimmed) > maxWordRunes {
		return store.ScoredWord{}, nil, errors.New("word is too long")
	}
	if len(req.NextWords) > maxNextWordsInRequest {
		return store.ScoredWord{}, nil, errors.New("nextWords is too long")
	}

	nextWords = make([]store.ScoredWord, 0, len(req.NextWords))
	for _, raw := range req.NextWords {
		// Ignore empty strings in array like ["", " "]
		var rawStr string
		if json.Unmarshal(raw, &rawStr) == nil && strings.TrimSpace(rawStr) == "" {
			continue
		}
		sw, ok := store.ParseScoredWordItem(raw)
		if !ok {
			return store.ScoredWord{}, nil, errors.New("invalid nextWords entry")
		}
		if utf8.RuneCountInString(sw.Word) > maxWordRunes {
			return store.ScoredWord{}, nil, errors.New("nextWords entry is too long")
		}
		nextWords = append(nextWords, sw)
	}

	return store.ScoredWord{Word: trimmed, Order: req.Order}, nextWords, nil
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
