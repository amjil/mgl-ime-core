package store

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"m-v-k/backend/internal/word/mongol"
)

type JSONStore struct {
	candidateMap map[string][]ScoredWord
	nextMap      map[string][]ScoredWord
	user         *userOverlay
}

func NewJSONStore(fullJSONPath, nextJSONPath string) (*JSONStore, error) {
	return NewJSONStoreWithUser(fullJSONPath, nextJSONPath, "", "")
}

func NewJSONStoreWithUser(fullJSONPath, nextJSONPath, userFullPath, userNextPath string) (*JSONStore, error) {
	candidateMap, err := loadWordMap(fullJSONPath)
	if err != nil {
		return nil, fmt.Errorf("load full json (%s): %w", fullJSONPath, err)
	}

	nextMap, err := loadWordMap(nextJSONPath)
	if err != nil {
		return nil, fmt.Errorf("load next json (%s): %w", nextJSONPath, err)
	}

	var user *userOverlay
	if userFullPath != "" && userNextPath != "" {
		user, err = newUserOverlay(userFullPath, userNextPath)
		if err != nil {
			return nil, err
		}
	}

	return &JSONStore{
		candidateMap: candidateMap,
		nextMap:      nextMap,
		user:         user,
	}, nil
}

func (s *JSONStore) GetCandidates(word string) ([]ScoredWord, bool) {
	return s.lookupMerged(word, s.candidateMap, func(key string) ([]ScoredWord, bool) {
		if s.user == nil {
			return nil, false
		}
		words, ok := s.user.getCandidates(key)
		if !ok {
			return nil, false
		}
		return scoredFromUserWords(words), true
	})
}

func (s *JSONStore) GetNextWords(word string) ([]ScoredWord, bool) {
	return s.lookupMerged(word, s.nextMap, func(key string) ([]ScoredWord, bool) {
		if s.user == nil {
			return nil, false
		}
		words, ok := s.user.getNextWords(key)
		if !ok {
			return nil, false
		}
		return scoredFromUserWords(words), true
	})
}

func (s *JSONStore) lookupMerged(word string, baseMap map[string][]ScoredWord, overlayFn func(string) ([]ScoredWord, bool)) ([]ScoredWord, bool) {
	var baseAll []ScoredWord
	var overlayAll []ScoredWord
	found := false
	seenKeys := make(map[string]struct{}, 2)

	for _, key := range mongol.LookupKeyVariants(word) {
		if key == "" {
			continue
		}
		if _, done := seenKeys[key]; done {
			continue
		}
		seenKeys[key] = struct{}{}

		base, baseOK := baseMap[key]
		var overlay []ScoredWord
		overlayOK := false
		if overlayFn != nil {
			overlay, overlayOK = overlayFn(key)
		}
		if !baseOK && !overlayOK {
			continue
		}
		found = true
		if baseOK {
			baseAll = append(baseAll, base...)
		}
		if overlayOK {
			overlayAll = append(overlayAll, overlay...)
		}
	}

	if !found {
		return nil, false
	}
	return mergeScoredWordLists(baseAll, overlayAll), true
}

func (s *JSONStore) AddUserWord(keys []string, word string, nextWords []string) error {
	if s.user == nil {
		return fmt.Errorf("user dictionary is not configured")
	}
	return s.user.addUserWord(keys, word, nextWords)
}

// UserOverlayGeneration returns a monotonic value that increases whenever user
// overlay data changes (disk reload or in-process write).
func (s *JSONStore) UserOverlayGeneration() (uint64, bool) {
	if s.user == nil {
		return 0, false
	}
	return s.user.refreshGeneration(), true
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

func loadWordMap(path string) (map[string][]ScoredWord, error) {
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

	out := make(map[string][]ScoredWord)
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
