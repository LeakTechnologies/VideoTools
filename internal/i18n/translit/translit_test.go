package translit

import (
	"testing"
)

func TestRomanToSyllabics_Vowels(t *testing.T) {
	tests := []struct{ roman, syll string }{
		{"i", "ᐃ"}, {"ii", "ᐄ"},
		{"u", "ᐅ"}, {"uu", "ᐆ"},
		{"a", "ᐊ"}, {"aa", "ᐋ"},
	}
	for _, tc := range tests {
		got := RomanToSyllabics(tc.roman)
		if got != tc.syll {
			t.Errorf("RomanToSyllabics(%q) = %q; want %q", tc.roman, got, tc.syll)
		}
	}
}

func TestRomanToSyllabics_Consonants(t *testing.T) {
	tests := []struct{ roman, syll string }{
		{"pi", "ᐱ"}, {"pii", "ᐲ"}, {"pu", "ᐳ"}, {"puu", "ᐴ"}, {"pa", "ᐸ"}, {"paa", "ᐹ"}, {"p", "ᑉ"},
		{"ti", "ᑎ"}, {"tii", "ᑏ"}, {"tu", "ᑐ"}, {"tuu", "ᑑ"}, {"ta", "ᑕ"}, {"taa", "ᑖ"}, {"t", "ᑦ"},
		{"ki", "ᑭ"}, {"kii", "ᑮ"}, {"ku", "ᑯ"}, {"kuu", "ᑰ"}, {"ka", "ᑲ"}, {"kaa", "ᑳ"}, {"k", "ᒃ"},
		{"gi", "ᒋ"}, {"gii", "ᒌ"}, {"gu", "ᒍ"}, {"guu", "ᒎ"}, {"ga", "ᒐ"}, {"gaa", "ᒑ"}, {"g", "ᒡ"},
		{"mi", "ᒥ"}, {"mii", "ᒦ"}, {"mu", "ᒧ"}, {"muu", "ᒨ"}, {"ma", "ᒪ"}, {"maa", "ᒫ"}, {"m", "ᒻ"},
		{"ni", "ᓂ"}, {"nii", "ᓃ"}, {"nu", "ᓄ"}, {"nuu", "ᓅ"}, {"na", "ᓇ"}, {"naa", "ᓈ"}, {"n", "ᓐ"},
		{"si", "ᓯ"}, {"sii", "ᓰ"}, {"su", "ᓱ"}, {"suu", "ᓲ"}, {"sa", "ᓴ"}, {"saa", "ᓵ"}, {"s", "ᔅ"},
		{"li", "ᓕ"}, {"lii", "ᓖ"}, {"lu", "ᓗ"}, {"luu", "ᓘ"}, {"la", "ᓚ"}, {"laa", "ᓛ"}, {"l", "ᓪ"},
		{"ji", "ᔨ"}, {"jii", "ᔩ"}, {"ju", "ᔪ"}, {"juu", "ᔫ"}, {"ja", "ᔭ"}, {"jaa", "ᔮ"}, {"j", "ᔾ"},
		{"vi", "ᕕ"}, {"vii", "ᕖ"}, {"vu", "ᕗ"}, {"vuu", "ᕘ"}, {"va", "ᕙ"}, {"vaa", "ᕚ"}, {"v", "ᕝ"},
		{"ri", "ᕆ"}, {"rii", "ᕇ"}, {"ru", "ᕈ"}, {"ruu", "ᕉ"}, {"ra", "ᕋ"}, {"raa", "ᕌ"}, {"r", "ᕐ"},
		{"qi", "ᕿ"}, {"qii", "ᖀ"}, {"qu", "ᖁ"}, {"quu", "ᖂ"}, {"qa", "ᖃ"}, {"qaa", "ᖄ"}, {"q", "ᖅ"},
		{"ngi", "ᖏ"}, {"ngii", "ᖐ"}, {"ngu", "ᖑ"}, {"nguu", "ᖒ"}, {"nga", "ᖓ"}, {"ngaa", "ᖔ"}, {"ng", "ᖕ"},
		{"nngi", "ᙱ"}, {"nngii", "ᙲ"}, {"nngu", "ᙳ"}, {"nnguu", "ᙴ"}, {"nnga", "ᙵ"}, {"nngaa", "ᙶ"}, {"nng", "ᖖ"},
		{"lhi", "ᖠ"}, {"lhii", "ᖡ"}, {"lhu", "ᖢ"}, {"lhuu", "ᖣ"}, {"lha", "ᖤ"}, {"lhaa", "ᖥ"}, {"lh", "ᖦ"},
		{"h", "ᕼ"},
	}
	for _, tc := range tests {
		got := RomanToSyllabics(tc.roman)
		if got != tc.syll {
			t.Errorf("RomanToSyllabics(%q) = %q; want %q", tc.roman, got, tc.syll)
		}
	}
}

func TestSyllabicsToRoman_All(t *testing.T) {
	tests := []struct{ syll, roman string }{
		{"ᐃ", "i"}, {"ᐄ", "ii"}, {"ᐅ", "u"}, {"ᐆ", "uu"}, {"ᐊ", "a"}, {"ᐋ", "aa"},
		{"ᐱ", "pi"}, {"ᐲ", "pii"}, {"ᐳ", "pu"}, {"ᐴ", "puu"}, {"ᐸ", "pa"}, {"ᐹ", "paa"}, {"ᑉ", "p"},
		{"ᑎ", "ti"}, {"ᑏ", "tii"}, {"ᑐ", "tu"}, {"ᑑ", "tuu"}, {"ᑕ", "ta"}, {"ᑖ", "taa"}, {"ᑦ", "t"},
		{"ᑭ", "ki"}, {"ᑮ", "kii"}, {"ᑯ", "ku"}, {"ᑰ", "kuu"}, {"ᑲ", "ka"}, {"ᑳ", "kaa"}, {"ᒃ", "k"},
		{"ᒋ", "gi"}, {"ᒌ", "gii"}, {"ᒍ", "gu"}, {"ᒎ", "guu"}, {"ᒐ", "ga"}, {"ᒑ", "gaa"}, {"ᒡ", "g"},
		{"ᒥ", "mi"}, {"ᒦ", "mii"}, {"ᒧ", "mu"}, {"ᒨ", "muu"}, {"ᒪ", "ma"}, {"ᒫ", "maa"}, {"ᒻ", "m"},
		{"ᓂ", "ni"}, {"ᓃ", "nii"}, {"ᓄ", "nu"}, {"ᓅ", "nuu"}, {"ᓇ", "na"}, {"ᓈ", "naa"}, {"ᓐ", "n"},
		{"ᓯ", "si"}, {"ᓰ", "sii"}, {"ᓱ", "su"}, {"ᓲ", "suu"}, {"ᓴ", "sa"}, {"ᓵ", "saa"}, {"ᔅ", "s"},
		{"ᓕ", "li"}, {"ᓖ", "lii"}, {"ᓗ", "lu"}, {"ᓘ", "luu"}, {"ᓚ", "la"}, {"ᓛ", "laa"}, {"ᓪ", "l"},
		{"ᔨ", "ji"}, {"ᔩ", "jii"}, {"ᔪ", "ju"}, {"ᔫ", "juu"}, {"ᔭ", "ja"}, {"ᔮ", "jaa"}, {"ᔾ", "j"},
		{"ᕕ", "vi"}, {"ᕖ", "vii"}, {"ᕗ", "vu"}, {"ᕘ", "vuu"}, {"ᕙ", "va"}, {"ᕚ", "vaa"}, {"ᕝ", "v"},
		{"ᕆ", "ri"}, {"ᕇ", "rii"}, {"ᕈ", "ru"}, {"ᕉ", "ruu"}, {"ᕋ", "ra"}, {"ᕌ", "raa"}, {"ᕐ", "r"},
		{"ᕿ", "qi"}, {"ᖀ", "qii"}, {"ᖁ", "qu"}, {"ᖂ", "quu"}, {"ᖃ", "qa"}, {"ᖄ", "qaa"}, {"ᖅ", "q"},
		{"ᖏ", "ngi"}, {"ᖐ", "ngii"}, {"ᖑ", "ngu"}, {"ᖒ", "nguu"}, {"ᖓ", "nga"}, {"ᖔ", "ngaa"}, {"ᖕ", "ng"},
		{"ᙱ", "nngi"}, {"ᙲ", "nngii"}, {"ᙳ", "nngu"}, {"ᙴ", "nnguu"}, {"ᙵ", "nnga"}, {"ᙶ", "nngaa"}, {"ᖖ", "nng"},
		{"ᖠ", "łi"}, {"ᖡ", "łii"}, {"ᖢ", "łu"}, {"ᖣ", "łuu"}, {"ᖤ", "ła"}, {"ᖥ", "łaa"}, {"ᖦ", "ł"},
		{"ᕼ", "h"},
	}
	for _, tc := range tests {
		got := SyllabicsToRoman(tc.syll)
		if got != tc.roman {
			t.Errorf("SyllabicsToRoman(%q) = %q; want %q", tc.syll, got, tc.roman)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	romans := []string{
		"inuktitut",
		"qaujisarniq",
		"titiraqtausimaujut",
		"pivaalliqtitiniq",
		"nalunaiqsilugit",
		"nuatausimaujut",
		"asijjiariaqtuq",
		"iksivautit",
		"iglumik",
	}
	for _, roman := range romans {
		syll := RomanToSyllabics(roman)
		back := SyllabicsToRoman(syll)
		if back != roman {
			t.Errorf("round trip for %q: got %q", roman, back)
		}
	}
}

func TestMixedCase(t *testing.T) {
	got := RomanToSyllabics("TAKUSAUGIT")
	if got != "ᑕᑯᓴᐅᒋᑦ" {
		t.Errorf("uppercase: got %q; want %q", got, "ᑕᑯᓴᐅᒋᑦ")
	}
}

func TestCompounds(t *testing.T) {
	tests := []struct{ syll, roman string }{
		{"ᕐᑭ", "rqi"},
		{"ᕐᑯ", "rqu"},
		{"ᕐᑲ", "rqa"},
		{"ᖅᑭ", "qqi"},
		{"ᖅᑯ", "qqu"},
		{"ᖅᑲ", "qqa"},
		{"ᖕᒋ", "ngi"},
		{"ᖕᒍ", "ngu"},
		{"ᖕᒐ", "nga"},
		{"ᖖᒋ", "nngi"},
		{"ᖖᒍ", "nngu"},
		{"ᖖᒐ", "nnga"},
	}
	for _, tc := range tests {
		got := SyllabicsToRoman(tc.syll)
		if got != tc.roman {
			t.Errorf("SyllabicsToRoman(%q) = %q; want %q", tc.syll, got, tc.roman)
		}
	}
}

func TestExistingStrings(t *testing.T) {
	// Verify round-trip for existing iu_latin.go strings
	tests := []string{
		"nuatausimaujut",
		"qaujisarniq",
		"sivulliqpaujut",
		"mikssannnut",
		"titiraqtausimaujut",
		"asijjiariaqtuq",
		"katitataulugu",
		"iggirrlugu",
		"asikkaarutaujut",
		"pivaalliqtitiniq",
		"nipiq",
		"sanarlugu",
		"nirunnasilugu",
		"nalunaiqsilugit",
		"tarvijaksalirijuq",
		"aaqiksuiniq",
	}
	for _, s := range tests {
		syll := RomanToSyllabics(s)
		back := SyllabicsToRoman(syll)
		if back != s {
			t.Errorf("round trip failed for %q: got %q", s, back)
		}
	}
}

func TestPassThrough(t *testing.T) {
	// Non-letter characters should pass through unchanged.
	got := RomanToSyllabics("123 !@#")
	if got != "123 !@#" {
		t.Errorf("passthrough: got %q; want %q", got, "123 !@#")
	}

	got2 := SyllabicsToRoman("abc 123")
	if got2 != "abc 123" {
		t.Errorf("s2r passthrough: got %q; want %q", got2, "abc 123")
	}
}

func TestEnglishLetters(t *testing.T) {
	// English letters that are also valid Inuktitut roman characters
	// get mapped to their syllabic equivalents. This is expected.
	got := RomanToSyllabics("hello")
	// h→ᕼ, e→e(passthrough), l→ᓪ, l→ᓪ, o→o(passthrough)
	if got == "hello" {
		t.Error("English letters h/l should map to syllabics")
	}
}

func TestEmpty(t *testing.T) {
	if got := RomanToSyllabics(""); got != "" {
		t.Errorf("empty: got %q", got)
	}
	if got := SyllabicsToRoman(""); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestRomanOnly(t *testing.T) {
	RomanOnly(true)
	defer RomanOnly(false)
	got := RomanToSyllabics("inuktitut")
	if got != "inuktitut" {
		t.Errorf("RomanOnly: got %q; want %q", got, "inuktitut")
	}
}

func TestIsSyllabics(t *testing.T) {
	if !IsSyllabics("ᐃ") {
		t.Error("IsSyllabics('ᐃ') should be true")
	}
	if IsSyllabics("abc") {
		t.Error("IsSyllabics('abc') should be false")
	}
	if !IsSyllabics("hello ᐃ world") {
		t.Error("IsSyllabics('hello ᐃ world') should be true")
	}
}

func TestSyllabicRatio(t *testing.T) {
	if r := SyllabicRatio("ᐃᐅᐊ"); r != 1.0 {
		t.Errorf("all syllabics: got %f", r)
	}
	if r := SyllabicRatio("abc"); r != 0.0 {
		t.Errorf("no syllabics: got %f", r)
	}
	if r := SyllabicRatio(""); r != 0.0 {
		t.Errorf("empty: got %f", r)
	}
}
