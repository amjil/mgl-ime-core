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

func TestParseScoredWordItemFormats(t *testing.T) {
	plain, ok := ParseScoredWordItem(jsonRaw(`"ᠤᠯᠤᠰ"`))
	if !ok || plain.Word != "ᠤᠯᠤᠰ" || plain.Order != 0 {
		t.Fatalf("plain = %#v ok=%v", plain, ok)
	}

	tuple, ok := ParseScoredWordItem(jsonRaw(`["ᠬᠡᠯᠡ", 50]`))
	if !ok || tuple.Word != "ᠬᠡᠯᠡ" || tuple.Order != 50 {
		t.Fatalf("tuple = %#v ok=%v", tuple, ok)
	}

	obj, ok := ParseScoredWordItem(jsonRaw(`{"word":"ᠪᠢᠴᠢᠭ","order":100}`))
	if !ok || obj.Word != "ᠪᠢᠴᠢᠭ" || obj.Order != 100 {
		t.Fatalf("obj = %#v ok=%v", obj, ok)
	}
}

func TestNormalizeUserFeedbackOrder(t *testing.T) {
	if got := NormalizeUserFeedbackOrder(50, 0, 2); got != UserFeedbackBaseOrder+50 {
		t.Fatalf("positive raw = %v", got)
	}
	if got := NormalizeUserFeedbackOrder(UserFeedbackBaseOrder+9, 0, 2); got != UserFeedbackBaseOrder+9 {
		t.Fatalf("already elevated = %v", got)
	}
	if got := NormalizeUserFeedbackOrder(0, 0, 2); got != UserFeedbackBaseOrder+2 {
		t.Fatalf("list position = %v", got)
	}
}

func TestMergeScoredWordListsUserTierBeatsHigherDictionaryOrder(t *testing.T) {
	base := []ScoredWord{{Word: "dict", Order: UserFeedbackBaseOrder + 999}}
	user := []ScoredWord{{Word: "feedback", Order: 1}}
	merged := mergeScoredWordLists(base, user)
	if len(merged) != 2 || merged[0].Word != "feedback" || merged[1].Word != "dict" {
		t.Fatalf("user tier should win regardless of raw order: %#v", merged)
	}
}
