package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	maxRequestBodyBytes = 4096
	maxWordRunes        = 256
)

type WordService interface {
	GetCandidates(word string) ([]string, error)
	GetNextWords(word string) ([]string, error)
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

	writeJSON(w, http.StatusOK, map[string][]string{"candidates": candidates})
}

func (h *WordHandler) HandleAddUserWord(w http.ResponseWriter, r *http.Request) {
	word, nextWords, err := parseAddUserWordRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.AddUserWord(word, nextWords); err != nil {
		if err.Error() == "user dictionary is not configured" {
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

	writeJSON(w, http.StatusOK, map[string][]string{"nextWords": nextWords})
}

const maxNextWordsInRequest = 64

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
