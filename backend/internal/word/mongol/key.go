package mongol

import "unicode/utf8"

// Unicode Mongolian code points (mongol_code / linguistics.cljd).
const (
	codeA   = 0x1820
	codeEE  = 0x1827
	codeNA  = 0x1828
	codeCHI = 0x1842

	codeNNBSP = 0x202F
	codeZWJ   = 0x200D

	codeMongolPunctStart = 0x1800
	codeMongolPunctEnd   = 0x1809
	codeNirugu           = 0x180A
)

// Mongolian variation selectors and MVS (matches linguistics.cljd fvs-or-mvs).
var fvsOrMVS = map[rune]struct{}{
	0x180B: {}, 0x180C: {}, 0x180D: {}, 0x180E: {}, 0x180F: {},
}

// isAbbrevIgnored drops vowels, MVS/FVS, NNBSP, ZWJ, nirugu, and Mongolian punctuation.
func isAbbrevIgnored(r rune) bool {
	if r == codeNNBSP || r == codeZWJ || r == codeNirugu {
		return true
	}
	if _, ok := fvsOrMVS[r]; ok {
		return true
	}
	if r >= codeMongolPunctStart && r <= codeMongolPunctEnd {
		return true
	}
	if !IsMongolianLetter(r) {
		return true
	}
	return false
}

func IsVowel(r rune) bool {
	return r >= codeA && r <= codeEE
}

func IsConsonant(r rune) bool {
	return r >= codeNA && r <= codeCHI
}

func IsMongolianLetter(r rune) bool {
	return IsVowel(r) || IsConsonant(r)
}

// AbbrevKey returns the consonant-skeleton lookup key used in full.json.
// Vowels are removed except a leading vowel at word start. MVS, FVS, NNBSP, ZWJ,
// nirugu, punctuation, and other non-letters are always removed.
func AbbrevKey(word string) string {
	if word == "" {
		return ""
	}

	var b []rune
	leadingVowelDone := false

	for _, r := range word {
		if isAbbrevIgnored(r) {
			continue
		}

		if IsVowel(r) {
			if !leadingVowelDone && len(b) == 0 {
				b = append(b, r)
				leadingVowelDone = true
			}
			continue
		}

		if IsConsonant(r) {
			if len(b) == 0 {
				leadingVowelDone = true
			}
			b = append(b, r)
		}
	}

	return string(b)
}

// CandidateKeys returns the lookup keys that should index word in user_full.json.
func CandidateKeys(word string) []string {
	word = trimWord(word)
	if word == "" {
		return nil
	}

	abbrev := AbbrevKey(word)
	if abbrev == "" || abbrev == word {
		return []string{word}
	}
	return []string{word, abbrev}
}

// NormalizeLookupKey strips MVS/FVS, NNBSP, ZWJ, punctuation, and other non-letters.
func NormalizeLookupKey(word string) string {
	word = trimWord(word)
	if word == "" {
		return ""
	}
	var b []rune
	for _, r := range word {
		if isAbbrevIgnored(r) {
			continue
		}
		b = append(b, r)
	}
	return string(b)
}

// LookupKeyVariants returns exact and normalized keys for dictionary lookup.
func LookupKeyVariants(word string) []string {
	word = trimWord(word)
	if word == "" {
		return nil
	}
	normalized := NormalizeLookupKey(word)
	if normalized == "" || normalized == word {
		return []string{word}
	}
	return []string{word, normalized}
}

func trimWord(word string) string {
	// Caller trims ASCII space; keep NNBSP inside words.
	for len(word) > 0 {
		r, size := utf8.DecodeRuneInString(word)
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			word = word[size:]
			continue
		}
		break
	}
	for len(word) > 0 {
		r, size := utf8.DecodeLastRuneInString(word)
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			word = word[:len(word)-size]
			continue
		}
		break
	}
	return word
}
