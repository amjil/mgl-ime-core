package store

import (
	"encoding/json"
	"testing"
)

func jsonRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}

func TestUserFeedbackOrderBeatsDictionaryScore(t *testing.T) {
	base := []ScoredWord{
		{Word: "ᠬᠡᠨ", Order: 36741720},
		{Word: "ᠬᠢᠨ", Order: 10},
	}
	user := scoredFromUserWords([]string{"ᠬᠠᠨᠢ", "ᠬᠡᠨ"})
	merged := mergeScoredWordLists(base, user)
	if len(merged) != 3 {
		t.Fatalf("merged = %#v", merged)
	}
	if merged[0].Word != "ᠬᠠᠨᠢ" || merged[0].Order < UserFeedbackBaseOrder {
		t.Fatalf("want feedback word first, got %#v", merged)
	}
	if merged[1].Word != "ᠬᠡᠨ" || merged[1].Order <= 36741720 {
		t.Fatalf("existing word should keep feedback order, got %#v", merged[1])
	}
	if merged[2].Word != "ᠬᠢᠨ" || merged[2].Order != 10 {
		t.Fatalf("dictionary tail = %#v", merged[2])
	}
}

func TestParseStringScore(t *testing.T) {
	got, err := parseWordList(jsonRaw(`[["ᠶᠢᠨ","57223"],["ᠶᠢ",16161]]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Order != 57223 || got[1].Order != 16161 {
		t.Fatalf("got %#v", got)
	}
}
