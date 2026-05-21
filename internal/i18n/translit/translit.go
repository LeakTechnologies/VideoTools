package translit

import (
	"strings"
	"sync"
	"unicode"
)

var (
	once      sync.Once
	romanOnly = false
)

// RomanOnly restricts the package to roman-orthography-only mode.
// When true, RomanToSyllabics returns its input unchanged.
func RomanOnly(v bool) { romanOnly = v }

func init() {
	once.Do(func() {
		initTables()
	})
}

// RomanToSyllabics converts a roman-orthography Inuktitut string to Unified
// Canadian Aboriginal Syllabics using ICI convention (no diphthong characters).
// Go printf format verbs (%s, %d, %v, %%, etc.) are passed through unchanged.
// Unknown characters are also passed through unchanged.
func RomanToSyllabics(s string) string {
	if romanOnly || s == "" {
		return s
	}
	s = strings.ToLower(s)

	var out strings.Builder
	out.Grow(len(s))

	i := 0
	for i < len(s) {
		// Pass through printf format verbs unchanged.
		if s[i] == '%' && i+1 < len(s) {
			out.WriteByte('%')
			out.WriteByte(s[i+1])
			i += 2
			continue
		}

		max := r2sMaxKey
		if remaining := len(s) - i; remaining < max {
			max = remaining
		}
		matched := false
		for l := max; l > 0; l-- {
			key := s[i : i+l]
			if val, ok := r2s[key]; ok {
				out.WriteString(val)
				i += l
				matched = true
				break
			}
		}
		if !matched {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

// SyllabicsToRoman converts a syllabics string to roman orthography.
// Handles compound sequences where r/q/ng/nng finals merge with
// following consonant+vowel syllables.
func SyllabicsToRoman(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	var out strings.Builder
	out.Grow(len(s))

	i := 0
	for i < len(runes) {
		// Try 2-rune compound match first
		if i+1 < len(runes) {
			key := string(runes[i]) + string(runes[i+1])
			if val, ok := s2rCompound[key]; ok {
				out.WriteString(val)
				i += 2
				continue
			}
		}

		// Single-rune match
		if val, ok := s2r[runes[i]]; ok {
			out.WriteString(val)
		} else {
			out.WriteRune(runes[i])
		}
		i++
	}

	return out.String()
}

// IsSyllabics reports whether s contains at least one syllabics character
// from the Unified Canadian Aboriginal Syllabics block (U+1400–U+167F).
func IsSyllabics(s string) bool {
	for _, r := range s {
		if r >= 0x1400 && r <= 0x167F {
			return true
		}
	}
	return false
}

// SyllabicRatio returns the fraction of word-class characters in s that
// are syllabics (0.0–1.0). Used to auto-detect the script of a string.
func SyllabicRatio(s string) float64 {
	var syll, total int
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			total++
			if r >= 0x1400 && r <= 0x167F {
				syll++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(syll) / float64(total)
}
