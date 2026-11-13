package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	"mgl-ime-core/backend/internal/word/mongol"
)

// ErrUserDictNotConfigured is returned when attempting to add user words without user overlay paths.
var ErrUserDictNotConfigured = errors.New("user dictionary is not configured")

const maxUserNextWords = 64

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
	return prependWords(list, []string{word}, 0)
}

func prependWords(list []string, words []string, limit int) []string {
	if len(words) == 0 {
		return cloneStrings(list)
	}
	seen := make(map[string]struct{}, len(list)+len(words))
	out := make([]string, 0, len(list)+len(words))
	appendUnique := func(w string) {
		if w == "" {
			return
		}
		if limit > 0 && len(out) >= limit {
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

// buildAbbrevIndex precomputes a secondary index mapping AbbrevKey(storedKey) -> merged words
// in deterministic key order, avoiding O(N) map scans and random map iteration order on lookup.
func buildAbbrevIndex(data map[string][]string) map[string][]string {
	if len(data) == 0 {
		return make(map[string][]string)
	}
	keys := make([]string, 0, len(data))
	for k, v := range data {
		if len(v) > 0 {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	idx := make(map[string][]string)
	for _, storedKey := range keys {
		abbrev := mongol.AbbrevKey(storedKey)
		if abbrev == "" || abbrev == storedKey {
			continue
		}
		// Keep earlier keys authoritative while deterministically merging fallback lists.
		idx[abbrev] = mergeWordLists(data[storedKey], idx[abbrev])
	}
	return idx
}

// updateAbbrevIndexForKeys refreshes only abbrev buckets touched by keys, preserving
// deterministic merge order without rebuilding unrelated index entries.
func updateAbbrevIndexForKeys(idx map[string][]string, data map[string][]string, keys []string) {
	if idx == nil || len(keys) == 0 {
		return
	}
	affected := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		abbrev := mongol.AbbrevKey(key)
		if abbrev == "" || abbrev == key {
			continue
		}
		affected[abbrev] = struct{}{}
	}
	if len(affected) == 0 {
		return
	}
	for abbrev := range affected {
		delete(idx, abbrev)
	}
	contrib := make([]string, 0, len(affected))
	for storedKey, values := range data {
		if len(values) == 0 {
			continue
		}
		abbrev := mongol.AbbrevKey(storedKey)
		if _, ok := affected[abbrev]; !ok || abbrev == storedKey {
			continue
		}
		contrib = append(contrib, storedKey)
	}
	slices.Sort(contrib)
	for _, storedKey := range contrib {
		abbrev := mongol.AbbrevKey(storedKey)
		idx[abbrev] = mergeWordLists(data[storedKey], idx[abbrev])
	}
}

type userOverlay struct {
	mu          sync.RWMutex
	fullPath    string
	nextPath    string
	full        map[string][]string
	next        map[string][]string
	fullAbbrev  map[string][]string
	nextAbbrev  map[string][]string
	fullModTime atomic.Int64
	nextModTime atomic.Int64
	generation  atomic.Uint64
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
	u.mu.Lock()
	defer u.mu.Unlock()
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
	u.fullAbbrev = buildAbbrevIndex(full)
	u.nextAbbrev = buildAbbrevIndex(next)
	u.fullModTime.Store(fileModTime(u.fullPath))
	u.nextModTime.Store(fileModTime(u.nextPath))
	u.generation.Add(1)
	return nil
}

// isStale checks file modification times without acquiring the write lock first.
func (u *userOverlay) isStale() bool {
	return fileModTime(u.fullPath) != u.fullModTime.Load() ||
		fileModTime(u.nextPath) != u.nextModTime.Load()
}

// refreshGeneration reloads overlay files only when disk modification times change.
func (u *userOverlay) refreshGeneration() uint64 {
	if u.isStale() {
		u.mu.Lock()
		if u.isStale() {
			_ = u.reloadLocked()
		}
		u.mu.Unlock()
	}
	return u.generation.Load()
}

func (u *userOverlay) reloadIfStaleLocked() error {
	if !u.isStale() {
		return nil
	}
	return u.reloadLocked()
}

func (u *userOverlay) getCandidates(key string) ([]string, bool) {
	return u.getOverlayList(key, false)
}

func (u *userOverlay) getNextWords(key string) ([]string, bool) {
	return u.getOverlayList(key, true)
}

// getOverlayList merges user overlay entries for lookupKey in O(1) using precomputed abbrev indexes.
func (u *userOverlay) getOverlayList(lookupKey string, isNext bool) ([]string, bool) {
	if u.isStale() {
		u.mu.Lock()
		if u.isStale() {
			if err := u.reloadLocked(); err != nil {
				u.mu.Unlock()
				return nil, false
			}
		}
		u.mu.Unlock()
	}

	u.mu.RLock()
	defer u.mu.RUnlock()

	data := u.full
	abbrevIdx := u.fullAbbrev
	if isNext {
		data = u.next
		abbrevIdx = u.nextAbbrev
	}

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
		// Keep earlier matched keys authoritative ahead of fallback keys.
		merged = mergeWordLists(values, merged)
		found = true
	}

	for _, k := range mongol.LookupKeyVariants(lookupKey) {
		tryKey(k)
	}
	tryKey(mongol.AbbrevKey(lookupKey))

	if fallback, ok := abbrevIdx[lookupKey]; ok && len(fallback) > 0 {
		merged = mergeWordLists(fallback, merged)
		found = true
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
	updateAbbrevIndexForKeys(u.fullAbbrev, u.full, keys)

	if len(nextWords) > 0 {
		nextKeys := keys
		if len(nextKeys) == 0 {
			nextKeys = mongol.CandidateKeys(word)
		}
		if len(nextKeys) == 0 {
			nextKeys = []string{word}
		}
		for _, key := range nextKeys {
			u.next[key] = prependWords(u.next[key], nextWords, maxUserNextWords)
		}
		updateAbbrevIndexForKeys(u.nextAbbrev, u.next, nextKeys)
	}

	if err := saveWordMap(u.fullPath, u.full); err != nil {
		return err
	}
	u.fullModTime.Store(fileModTime(u.fullPath))

	if len(nextWords) > 0 {
		if err := saveWordMap(u.nextPath, u.next); err != nil {
			return err
		}
		u.nextModTime.Store(fileModTime(u.nextPath))
	}
	u.generation.Add(1)
	return nil
}
