package mongol

import "testing"

func TestAbbrevKeyExamplesFromFullJSON(t *testing.T) {
	tests := []struct {
		word   string
		abbrev string
	}{
		{"ᠰᠡᠪᠡᠷᠡᠭᠡᠳᠡᠭ", "ᠰᠪᠷᠭᠳᠭ"},
		{"ᠣᠶᠣᠭᠤᠯᠴᠢᠬᠠᠭᠠᠳ", "ᠣᠶᠭᠯᠴᠬᠭᠳ"},
		{"ᠴᠢᠬᠢᠳᠡᠭᠦᠯᠦᠭᠡᠳ", "ᠴᠬᠳᠭᠯᠭᠳ"},
	}
	for _, tc := range tests {
		got := AbbrevKey(tc.word)
		if got != tc.abbrev {
			t.Fatalf("AbbrevKey(%q) = %q, want %q", tc.word, got, tc.abbrev)
		}
	}
}

func TestAbbrevKeyStripsMVSFVSAndControls(t *testing.T) {
	// Same letters as ᠰᠡᠪᠡᠷᠡᠭᠡᠳᠡᠭ with MVS/FVS/NNBSP/ZWJ inserted.
	word := "ᠰ\u180eᠡ\u180bᠪᠡ\u202fᠷ\u200dᠡᠭᠡ\u180dᠳᠡᠭ"
	got := AbbrevKey(word)
	want := "ᠰᠪᠷᠭᠳᠭ"
	if got != want {
		t.Fatalf("AbbrevKey with controls = %q, want %q", got, want)
	}
}

func TestNormalizeLookupKeyStripsControls(t *testing.T) {
	raw := "ᠪᠠᠷᠠᠭᠠᠨ\u180e"
	got := NormalizeLookupKey(raw)
	want := "ᠪᠠᠷᠠᠭᠠᠨ"
	if got != want {
		t.Fatalf("NormalizeLookupKey = %q, want %q", got, want)
	}
}

func TestCandidateKeysIncludesFullAndAbbrev(t *testing.T) {
	word := "ᠰᠡᠪᠡᠷᠡᠭᠡᠳᠡᠭ"
	keys := CandidateKeys(word)
	if len(keys) != 2 || keys[0] != word || keys[1] != "ᠰᠪᠷᠭᠳᠭ" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}
