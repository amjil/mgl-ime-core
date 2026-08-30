package store

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	return values, ok
}

func (s *JSONStore) GetNextWords(word string) ([]string, bool) {
	values, ok := s.nextMap[word]
	return values, ok
}

// ResolveDataPath finds name in dir, falling back to name+".gz".
func ResolveDataPath(dir, name string) (string, error) {
	candidates := []string{filepath.Join(dir, name)}
	if !strings.HasSuffix(strings.ToLower(name), ".gz") {
		candidates = append(candidates, filepath.Join(dir, name+".gz"))
	}

	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("data file %q not found in %s (tried %s)", name, dir, strings.Join(candidates, ", "))
}

func loadWordMap(path string) (map[string][]string, error) {
	reader, err := openMaybeGzip(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	tok, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected JSON object, got %v", tok)
	}

	out := make(map[string][]string)
	for decoder.More() {
		keyTok, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", keyTok)
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("decode key %q: %w", key, err)
		}

		words, err := parseWordList(raw)
		if err != nil {
			return nil, fmt.Errorf("decode key %q: %w", key, err)
		}
		out[key] = words
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return out, nil
}

func parseWordList(raw json.RawMessage) ([]string, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("expected list: %w", err)
	}

	words := make([]string, 0, len(items))
	for _, item := range items {
		if word, ok := unmarshalJSONString(item); ok {
			word = strings.TrimSpace(word)
			if word != "" {
				words = append(words, word)
			}
			continue
		}

		var tuple []json.RawMessage
		if err := json.Unmarshal(item, &tuple); err != nil || len(tuple) == 0 {
			continue
		}
		word, ok := unmarshalJSONString(tuple[0])
		if !ok {
			continue
		}
		word = strings.TrimSpace(word)
		if word != "" {
			words = append(words, word)
		}
	}

	return words, nil
}

func unmarshalJSONString(raw json.RawMessage) (string, bool) {
	var word string
	if err := json.Unmarshal(raw, &word); err != nil {
		return "", false
	}
	return word, true
}

func openMaybeGzip(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var header [2]byte
	n, err := file.Read(header[:])
	if err != nil && err != io.EOF {
		file.Close()
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}

	if n == 2 && header[0] == 0x1f && header[1] == 0x8b {
		gz, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		return &gzipReadCloser{gz: gz, file: file}, nil
	}

	return file, nil
}

type gzipReadCloser struct {
	gz   *gzip.Reader
	file *os.File
}

func (c *gzipReadCloser) Read(p []byte) (int, error) {
	return c.gz.Read(p)
}

func (c *gzipReadCloser) Close() error {
	err := c.gz.Close()
	if closeErr := c.file.Close(); err == nil {
		err = closeErr
	}
	return err
}
