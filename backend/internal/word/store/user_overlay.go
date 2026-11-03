package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"m-v-k/backend/internal/word/mongol"
)

func loadUserWordMap(path string) (map[string][]string, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]string), nil
		}
		return nil, err
	}
	ranked, err := loadWordMap(path)
	if err != nil {
		return nil, err
	}
	return scoredMapToWords(ranked), nil
}

func scoredMapToWords(ranked map[string][]ScoredWord) map[string][]string {
	out := make(map[string][]string, len(ranked))
	for key, list := range ranked {
		words := make([]string, 0, len(list))
		for _, sw := range list {
			if sw.Word != "" {
				words = append(words, sw.Word)
			}
		}
		out[key] = words
	}
	return out
}

func saveWordMap(path string, data map[string][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func mergeWordLists(base, overlay []string) []string {
	if len(overlay) == 0 {
		return cloneStrings(base)
	}
	if len(base) == 0 {
		return cloneStrings(overlay)
	}

	seen := make(map[string]struct{}, len(base)+len(overlay))
	out := make([]string, 0, len(base)+len(overlay))

	appendUnique := func(words []string) {
		for _, w := range words {
			if w == "" {
				continue
			}
			if _, ok := seen[w]; ok {
				continue
			}
			seen[w] = struct{}{}
			out = append(out, w)
		}
	}

	appendUnique(overlay)
	appendUnique(base)
	return out
}

func prependWord(list []string, word string) []string {
	return prependWords(list, []string{word})
}

func prependWords(list []string, words []string) []string {
	if len(words) == 0 {
		return cloneStrings(list)
	}
	seen := make(map[string]struct{}, len(list)+len(words))
	out := make([]string, 0, len(list)+len(words))

	appendUnique := func(w string) {
		if w == "" {
			return
		}
		if _, ok := seen[w]; ok {
			return
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}

	for _, w := range words {
		appendUnique(w)
	}
	for _, w := range list {
		appendUnique(w)
	}
	return out
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

type userOverlay struct {
	mu          sync.RWMutex
	fullPath    string
	nextPath    string
	full        map[string][]string
	next        map[string][]string
	fullModTime int64
	nextModTime int64
	generation  uint64
}

func fileModTime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

func newUserOverlay(fullPath, nextPath string) (*userOverlay, error) {
	u := &userOverlay{
		fullPath: fullPath,
		nextPath: nextPath,
	}
	if err := u.reloadLocked(); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *userOverlay) reloadLocked() error {
	full, err := loadUserWordMap(u.fullPath)
	if err != nil {
		return fmt.Errorf("load user full json (%s): %w", u.fullPath, err)
	}
	next, err := loadUserWordMap(u.nextPath)
	if err != nil {
		return fmt.Errorf("load user next json (%s): %w", u.nextPath, err)
	}
	u.full = full
	u.next = next
	u.fullModTime = fileModTime(u.fullPath)
	u.nextModTime = fileModTime(u.nextPath)
	u.generation++
	return nil
}

// refreshGeneration reloads overlay files when stale and returns the current generation.
func (u *userOverlay) refreshGeneration() uint64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	_ = u.reloadIfStaleLocked()
	return u.generation
}

func (u *userOverlay) reloadIfStaleLocked() error {
	fullMod := fileModTime(u.fullPath)
	nextMod := fileModTime(u.nextPath)
	if fullMod == u.fullModTime && nextMod == u.nextModTime {
		return nil
	}
	return u.reloadLocked()
}

func (u *userOverlay) reloadIfStale() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.reloadIfStaleLocked()
}

func (u *userOverlay) getCandidates(key string) ([]string, bool) {
	return u.getOverlayList(key, u.full)
}

func (u *userOverlay) getNextWords(key string) ([]string, bool) {
	return u.getOverlayList(key, u.next)
}

// getOverlayList merges user overlay entries for lookupKey. Besides exact key
// hits, entries stored only under a full word key still match abbrev lookups
// (AbbrevKey(storedKey) == lookupKey).
func (u *userOverlay) getOverlayList(lookupKey string, data map[string][]string) ([]string, bool) {
	if err := u.reloadIfStale(); err != nil {
		return nil, false
	}
	u.mu.RLock()
	defer u.mu.RUnlock()

	var merged []string
	found := false
	seenKeys := make(map[string]struct{}, 4)

	tryKey := func(k string) {
		if k == "" {
			return
		}
		if _, done := seenKeys[k]; done {
			return
		}
		seenKeys[k] = struct{}{}
		values, ok := data[k]
		if !ok || len(values) == 0 {
			return
		}
		merged = mergeWordLists(values, merged)
		found = true
	}

	for _, k := range mongol.LookupKeyVariants(lookupKey) {
		tryKey(k)
	}
	tryKey(mongol.AbbrevKey(lookupKey))

	for storedKey, values := range data {
		if len(values) == 0 {
			continue
		}
		if _, done := seenKeys[storedKey]; done {
			continue
		}
		if mongol.AbbrevKey(storedKey) == lookupKey {
			merged = mergeWordLists(values, merged)
			found = true
		}
	}

	if !found {
		return nil, false
	}
	return merged, true
}

func (u *userOverlay) addUserWord(keys []string, word string, nextWords []string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.reloadIfStaleLocked(); err != nil {
		return err
	}

	for _, key := range keys {
		u.full[key] = prependWord(u.full[key], word)
	}
	if len(nextWords) > 0 {
		nextKeys := keys
		if len(nextKeys) == 0 {
			nextKeys = mongol.CandidateKeys(word)
		}
		if len(nextKeys) == 0 {
			nextKeys = []string{word}
		}
		for _, key := range nextKeys {
			u.next[key] = prependWords(u.next[key], nextWords)
		}
	}

	if err := saveWordMap(u.fullPath, u.full); err != nil {
		return err
	}
	u.fullModTime = fileModTime(u.fullPath)
	if len(nextWords) > 0 {
		if err := saveWordMap(u.nextPath, u.next); err != nil {
			return err
		}
		u.nextModTime = fileModTime(u.nextPath)
	}
	u.generation++
	return nil
}
