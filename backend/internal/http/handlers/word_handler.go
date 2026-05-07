package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type WordService interface {
	GetCandidates(word string) ([]string, error)
	GetNextWords(word string) ([]string, error)
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

func parseWordRequest(r *http.Request) (string, error) {
	defer r.Body.Close()

	var req wordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", errors.New("invalid json body")
	}

	word := strings.TrimSpace(req.Word)
	if word == "" {
		return "", errors.New("word is required")
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
