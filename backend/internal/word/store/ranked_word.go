package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// UserFeedbackBaseOrder is assigned to words the user just submitted.
// Dictionary scores in full.json / next.json stay far below this (the largest
// observed values are tens of millions), so a client that sorts by descending
// order shows feedback first.
const UserFeedbackBaseOrder = 1e15

// ScoredWord is a candidate or next-word entry with a sortable weight.
type ScoredWord struct {
	Word  string
	Order float64
}

// MarshalJSON serializes ScoredWord as a 2-tuple ["word", order].
func (sw ScoredWord) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(len(sw.Word) + 16)
	buf.WriteByte('[')
	wordJSON, err := json.Marshal(sw.Word)
	if err != nil {
		return nil, err
	}
	buf.Write(wordJSON)
	buf.WriteByte(',')
	buf.WriteString(strconv.FormatFloat(sw.Order, 'f', -1, 64))
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// NormalizeUserFeedbackOrder guarantees that a user-supplied rank sits in the
// authoritative feedback tier (>= UserFeedbackBaseOrder).
func NormalizeUserFeedbackOrder(rawOrder float64, index int, total int) float64 {
	if rawOrder >= UserFeedbackBaseOrder {
		return rawOrder
	}
	if rawOrder > 0 {
		return UserFeedbackBaseOrder + rawOrder
	}
	// When no explicit score is given, preserve list order above UserFeedbackBaseOrder.
	offset := float64(total - index)
	if offset < 0 {
		offset = 0
	}
	return UserFeedbackBaseOrder + offset
}

func scoredFromUserWords(words []string) []ScoredWord {
	out := make([]ScoredWord, 0, len(words))
	for i, w := range words {
		if w == "" {
			continue
		}
		out = append(out, ScoredWord{
			Word:  w,
			Order: UserFeedbackBaseOrder - float64(i),
		})
	}
	return out
}

type scoredAcc struct {
	sw   ScoredWord
	user bool
}

// mergeScoredWordLists keeps user-feedback entries strictly ahead of base dictionary
// entries, sorted by descending feedback order. Dictionary duplicates keep the higher score.
func mergeScoredWordLists(base, overlay []ScoredWord) []ScoredWord {
	index := make(map[string]int, len(base)+len(overlay))
	out := make([]scoredAcc, 0, len(base)+len(overlay))

	consider := func(sw ScoredWord, user bool) {
		if sw.Word == "" {
			return
		}
		if i, ok := index[sw.Word]; ok {
			cur := &out[i]
			if cur.user {
				if user && sw.Order > cur.sw.Order {
					cur.sw.Order = sw.Order
				}
				return
			}
			if user || sw.Order > cur.sw.Order {
				cur.sw = sw
				cur.user = user
			}
			return
		}
		index[sw.Word] = len(out)
		out = append(out, scoredAcc{sw: sw, user: user})
	}

	for _, sw := range overlay {
		consider(sw, true)
	}
	for _, sw := range base {
		consider(sw, false)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].user != out[j].user {
			return out[i].user
		}
		return out[i].sw.Order > out[j].sw.Order
	})

	ranked := make([]ScoredWord, len(out))
	for i := range out {
		ranked[i] = out[i].sw
	}
	return ranked
}

// mergeUserScoredLists merges two user-feedback lists by descending order.
// Both sides are treated as feedback so the user-vs-dictionary tier does not apply.
func mergeUserScoredLists(primary, secondary []ScoredWord) []ScoredWord {
	index := make(map[string]int, len(primary)+len(secondary))
	out := make([]ScoredWord, 0, len(primary)+len(secondary))

	consider := func(sw ScoredWord) {
		if sw.Word == "" {
			return
		}
		if i, ok := index[sw.Word]; ok {
			if sw.Order > out[i].Order {
				out[i].Order = sw.Order
			}
			return
		}
		index[sw.Word] = len(out)
		out = append(out, sw)
	}

	for _, sw := range primary {
		consider(sw)
	}
	for _, sw := range secondary {
		consider(sw)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Order > out[j].Order
	})
	return out
}

func parseWordList(raw json.RawMessage) ([]ScoredWord, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("expected list: %w", err)
	}

	words := make([]ScoredWord, 0, len(items))
	for _, item := range items {
		if sw, ok := ParseScoredWordItem(item); ok {
			words = append(words, sw)
		}
	}
	return words, nil
}

// ParseScoredWordItem parses either "word", ["word", score], or {"word":"...", "order": score}.
func ParseScoredWordItem(item json.RawMessage) (ScoredWord, bool) {
	if word, ok := unmarshalJSONString(item); ok {
		word = strings.TrimSpace(word)
		if word == "" {
			return ScoredWord{}, false
		}
		return ScoredWord{Word: word, Order: 0}, true
	}

	var tuple []json.RawMessage
	if err := json.Unmarshal(item, &tuple); err == nil && len(tuple) > 0 {
		word, ok := unmarshalJSONString(tuple[0])
		if !ok {
			return ScoredWord{}, false
		}
		word = strings.TrimSpace(word)
		if word == "" {
			return ScoredWord{}, false
		}
		order := 0.0
		if len(tuple) > 1 {
			if n, ok := parseScore(tuple[1]); ok {
				order = n
			}
		}
		return ScoredWord{Word: word, Order: order}, true
	}

	var obj struct {
		Word  string          `json:"word"`
		Order json.RawMessage `json:"order"`
		Score json.RawMessage `json:"score"`
	}
	if err := json.Unmarshal(item, &obj); err == nil {
		word := strings.TrimSpace(obj.Word)
		if word == "" {
			return ScoredWord{}, false
		}
		order := 0.0
		if len(obj.Order) > 0 {
			if n, ok := parseScore(obj.Order); ok {
				order = n
			}
		} else if len(obj.Score) > 0 {
			if n, ok := parseScore(obj.Score); ok {
				order = n
			}
		}
		return ScoredWord{Word: word, Order: order}, true
	}

	return ScoredWord{}, false
}

func parseScore(raw json.RawMessage) (float64, bool) {
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
