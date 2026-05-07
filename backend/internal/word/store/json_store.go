package store

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type JSONStore struct {
	candidateMap map[string][]string
	nextMap      map[string][]string
}

func NewJSONStore(fullJSONPath, nextJSONPath string) (*JSONStore, error) {
	candidateMap, err := loadWordMap(fullJSONPath)
	if err != nil {
		return nil, fmt.Errorf("load full json (%s): %w", fullJSONPath, err)
	}

	nextMap, err := loadWordMap(nextJSONPath)
	if err != nil {
		return nil, fmt.Errorf("load next json (%s): %w", nextJSONPath, err)
	}

	return &JSONStore{
		candidateMap: candidateMap,
		nextMap:      nextMap,
	}, nil
}

func (s *JSONStore) GetCandidates(word string) ([]string, bool) {
	values, ok := s.candidateMap[word]
	if !ok {
		return nil, false
	}
	return clone(values), true
}

func (s *JSONStore) GetNextWords(word string) ([]string, bool) {
	values, ok := s.nextMap[word]
	if !ok {
		return nil, false
	}
	return clone(values), true
}

func loadWordMap(path string) (map[string][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	raw := map[string]any{}
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}

	out := make(map[string][]string, len(raw))
	for key, value := range raw {
		words, err := normalizeWordList(value)
		if err != nil {
			return nil, fmt.Errorf("decode key %q: %w", key, err)
		}
		out[key] = words
	}

	return out, nil
}

func normalizeWordList(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list, got %T", value)
	}

	words := make([]string, 0, len(items))
	for _, item := range items {
		switch entry := item.(type) {
		case string:
			word := strings.TrimSpace(entry)
			if word == "" {
				continue
			}
			words = append(words, word)
		case []any:
			// Support format like: ["word", 0.123]
			if len(entry) == 0 {
				continue
			}
			word, ok := entry[0].(string)
			if !ok {
				// Some source rows contain invalid/null first column; skip them.
				continue
			}
			word = strings.TrimSpace(word)
			if word == "" {
				continue
			}
			words = append(words, word)
		default:
			// Skip invalid item and keep loading remaining rows.
			continue
		}
	}

	return words, nil
}

func clone(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
