package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"mgl-ime-core/backend/internal/word/mongol"
)

// ErrUserDictNotConfigured is returned when attempting to add user words without user overlay paths.
var ErrUserDictNotConfigured = errors.New("user dictionary is not configured")

const maxUserNextWords = 64

func loadUserScoredMap(path string) (map[string][]ScoredWord, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]ScoredWord), nil
		}
		return nil, err
	}
	ranked, err := loadWordMap(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]ScoredWord, len(ranked))
	for key, list := range ranked {
		words := make([]ScoredWord, 0, len(list))
		for i, sw := range list {
			if sw.Word == "" {
				continue
			}
			order := sw.Order
			if order <= 0 {
				// Assign descending feedback order based on stored list position.
				order = UserFeedbackBaseOrder - float64(i)
			} else if order < UserFeedbackBaseOrder {
				order = UserFeedbackBaseOrder + order
			}
			words = append(words, ScoredWord{
				Word:  sw.Word,
				Order: order,
			})
		}
		// Ensure the loaded list is sorted by descending order.
		sort.SliceStable(words, func(i, j int) bool {
			return words[i].Order > words[j].Order
		})
		out[key] = words
	}
	return out, nil
}

// saveUserScoredMap writes the sorted word list as strings so existing tools and tests
// expecting map[string][]string in user_full.json / user_next.json remain compatible,
// while preserving the exact feedback rank order in list positions.
func saveUserScoredMap(path string, data map[string][]ScoredWord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	plain := make(map[string][]string, len(data))
	for k, list := range data {
		words := make([]string, 0, len(list))
		for _, sw := range list {
			if sw.Word != "" {
				words = append(words, sw.Word)
			}
		}
		plain[k] = words
	}

	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(plain); err != nil {
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

// prependScoredWords merges incoming feedback entries with existing user entries.
// New feedback with explicit Order is ranked by Order; feedback without explicit Order (Order <= 0)
// is boosted above existing entries so the latest feedback always appears first.
func prependScoredWords(existing []ScoredWord, incoming []ScoredWord, limit int) []ScoredWord {
	if len(incoming) == 0 {
		return cloneScoredWords(existing)
	}

	// Find current max order among existing entries to boost un-scored new feedback above it.
	maxExisting := float64(UserFeedbackBaseOrder)
	for _, sw := range existing {
		if sw.Order > maxExisting {
			maxExisting = sw.Order
		}
	}

	normalizedIncoming := make([]ScoredWord, 0, len(incoming))
	n := len(incoming)
	for i, sw := range incoming {
		if sw.Word == "" {
			continue
		}
		order := sw.Order
		if order <= 0 {
			// Latest feedback without explicit score gets placed strictly ahead of existing entries.
			order = maxExisting + float64(n-i)
		} else if order < UserFeedbackBaseOrder {
			order = UserFeedbackBaseOrder + order
		}
		normalizedIncoming = append(normalizedIncoming, ScoredWord{
			Word:  sw.Word,
			Order: order,
		})
	}

	merged := mergeUserScoredLists(normalizedIncoming, existing)
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

func cloneScoredWords(values []ScoredWord) []ScoredWord {
	if len(values) == 0 {
		return nil
	}
	out := make([]ScoredWord, len(values))
	copy(out, values)
	return out
}

// buildAbbrevScoredIndex precomputes AbbrevKey(storedKey) -> merged ScoredWords in deterministic order.
func buildAbbrevScoredIndex(data map[string][]ScoredWord) map[string][]ScoredWord {
	if len(data) == 0 {
		return make(map[string][]ScoredWord)
	}
	keys := make([]string, 0, len(data))
	for k, v := range data {
		if len(v) > 0 {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	idx := make(map[string][]ScoredWord)
	for _, storedKey := range keys {
		abbrev := mongol.AbbrevKey(storedKey)
		if abbrev == "" || abbrev == storedKey {
			continue
		}
		idx[abbrev] = mergeUserScoredLists(idx[abbrev], data[storedKey])
	}
	return idx
}

type userOverlay struct {
	mu          sync.RWMutex
	fullPath    string
	nextPath    string
	full        map[string][]ScoredWord
	next        map[string][]ScoredWord
	fullAbbrev  map[string][]ScoredWord
	nextAbbrev  map[string][]ScoredWord
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
	full, err := loadUserScoredMap(u.fullPath)
	if err != nil {
		return fmt.Errorf("load user full json (%s): %w", u.fullPath, err)
	}
	next, err := loadUserScoredMap(u.nextPath)
	if err != nil {
		return fmt.Errorf("load user next json (%s): %w", u.nextPath, err)
	}
	u.full = full
	u.next = next
	u.fullAbbrev = buildAbbrevScoredIndex(full)
	u.nextAbbrev = buildAbbrevScoredIndex(next)
	u.fullModTime.Store(fileModTime(u.fullPath))
	u.nextModTime.Store(fileModTime(u.nextPath))
	u.generation.Add(1)
	return nil
}

func (u *userOverlay) isStale() bool {
	return fileModTime(u.fullPath) != u.fullModTime.Load() ||
		fileModTime(u.nextPath) != u.nextModTime.Load()
}

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

func (u *userOverlay) getCandidates(key string) ([]ScoredWord, bool) {
	return u.getOverlayList(key, false)
}

func (u *userOverlay) getNextWords(key string) ([]ScoredWord, bool) {
	return u.getOverlayList(key, true)
}

// getOverlayList merges user overlay entries for lookupKey in O(1), ensuring direct key
// hits outrank abbreviated fallback hits.
func (u *userOverlay) getOverlayList(lookupKey string, isNext bool) ([]ScoredWord, bool) {
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

	var merged []ScoredWord
	found := false
	seenKeys := make(map[string]struct{}, 4)

	// tierPenalty ensures exact key matches strictly outrank normalized/abbrev fallback matches
	// when both have default feedback scores.
	tier := 0.0
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
		adjusted := make([]ScoredWord, len(values))
		for i, sw := range values {
			adjusted[i] = ScoredWord{
				Word:  sw.Word,
				Order: sw.Order - tier,
			}
		}
		tier += 1000.0
		merged = mergeUserScoredLists(merged, adjusted)
		found = true
	}

	for _, k := range mongol.LookupKeyVariants(lookupKey) {
		tryKey(k)
	}
	tryKey(mongol.AbbrevKey(lookupKey))

	if fallback, ok := abbrevIdx[lookupKey]; ok && len(fallback) > 0 {
		// Reuse current tier so sole abbrev-index hits keep full feedback order,
		// while hits after a direct key stay strictly below that key's entries.
		adjusted := make([]ScoredWord, len(fallback))
		for i, sw := range fallback {
			adjusted[i] = ScoredWord{
				Word:  sw.Word,
				Order: sw.Order - tier,
			}
		}
		merged = mergeUserScoredLists(merged, adjusted)
		found = true
	}

	if !found {
		return nil, false
	}
	return merged, true
}

func (u *userOverlay) addUserWord(keys []string, word ScoredWord, nextWords []ScoredWord) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.reloadIfStaleLocked(); err != nil {
		return err
	}
	for _, key := range keys {
		u.full[key] = prependScoredWords(u.full[key], []ScoredWord{word}, 0)
	}
	u.fullAbbrev = buildAbbrevScoredIndex(u.full)

	if len(nextWords) > 0 {
		nextKeys := keys
		if len(nextKeys) == 0 {
			nextKeys = mongol.CandidateKeys(word.Word)
		}
		if len(nextKeys) == 0 {
			nextKeys = []string{word.Word}
		}
		for _, key := range nextKeys {
			u.next[key] = prependScoredWords(u.next[key], nextWords, maxUserNextWords)
		}
		u.nextAbbrev = buildAbbrevScoredIndex(u.next)
	}

	if err := saveUserScoredMap(u.fullPath, u.full); err != nil {
		return err
	}
	u.fullModTime.Store(fileModTime(u.fullPath))

	if len(nextWords) > 0 {
		if err := saveUserScoredMap(u.nextPath, u.next); err != nil {
			return err
		}
		u.nextModTime.Store(fileModTime(u.nextPath))
	}
	u.generation.Add(1)
	return nil
}
