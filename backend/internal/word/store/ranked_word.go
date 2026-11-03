package store

import (
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

// mergeScoredWordLists keeps user-feedback order authoritative, then sorts by
// descending order. Dictionary duplicates keep the higher score.
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
		return out[i].sw.Order > out[j].sw.Order
	})

	ranked := make([]ScoredWord, len(out))
	for i := range out {
		ranked[i] = out[i].sw
	}
	return ranked
}

func parseWordList(raw json.RawMessage) ([]ScoredWord, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("expected list: %w", err)
	}

	words := make([]ScoredWord, 0, len(items))
	for _, item := range items {
		if word, ok := unmarshalJSONString(item); ok {
			word = strings.TrimSpace(word)
			if word != "" {
				words = append(words, ScoredWord{Word: word, Order: 0})
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
		if word == "" {
			continue
		}
		order := 0.0
		if len(tuple) > 1 {
			if n, ok := parseScore(tuple[1]); ok {
				order = n
			}
		}
		words = append(words, ScoredWord{Word: word, Order: order})
	}

	return words, nil
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
