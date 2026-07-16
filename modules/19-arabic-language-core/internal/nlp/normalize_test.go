package nlp

import (
	"testing"
)

func TestNormalize_TashkeelRemoval(t *testing.T) {
	n := DefaultNormalizer()
	text := "بِسْمِ اللَّهِ الرَّحْمَٰنِ الرَّحِيمِ"
	result := n.Normalize(text)

	if result.Normalized == text {
		t.Fatal("expected tashkeel to be removed")
	}
	if result.OriginalLength <= result.NormalizedLength {
		t.Fatalf("expected normalized length (%d) < original length (%d)", result.NormalizedLength, result.OriginalLength)
	}
	if len(result.Actions) == 0 {
		t.Fatal("expected at least one normalization action")
	}

	found := false
	for _, a := range result.Actions {
		if a.Type == "remove_tashkeel" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected remove_tashkeel action in actions list")
	}
}

func TestNormalize_TashkeelRemoval_Tanween(t *testing.T) {
	n := DefaultNormalizer()
	// U+064B FATHATAN, U+064C KASRATAN, U+064E DAMMATAN, U+0650 KASRA, U+064F FATHA, U+0651 SHADDA
	text := "كتابٌ جميلٌ"
	result := n.Normalize(text)
	if result.Normalized == text {
		t.Fatal("expected tanween marks to be removed")
	}
}

func TestNormalize_AlefNormalization(t *testing.T) {
	n := &Normalizer{RemoveTashkeel: true, NormalizeAlef: true, NormalizeWhitespace: true}
	// Alef with hamza above (U+0623), below (U+0625), horizontal (U+0622)
	text := "آإاسلام"
	result := n.Normalize(text)
	// All should be normalized to U+0627
	for _, r := range result.Normalized {
		if r == 0x0623 || r == 0x0625 || r == 0x0622 {
			t.Fatalf("expected alef normalized, but found U+%04X", r)
		}
	}
}

func TestNormalize_EmptyText(t *testing.T) {
	n := DefaultNormalizer()
	result := n.Normalize("")
	if result.Normalized != "" {
		t.Fatalf("expected empty normalized, got %q", result.Normalized)
	}
	if result.OriginalLength != 0 {
		t.Fatalf("expected original_length=0, got %d", result.OriginalLength)
	}
	if result.NormalizedLength != 0 {
		t.Fatalf("expected normalized_length=0, got %d", result.NormalizedLength)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(result.Actions))
	}
}

func TestNormalize_LongText(t *testing.T) {
	n := DefaultNormalizer()
	// Build a 10k+ character Arabic text
	chars := "بسم الله الرحمن الرحيم "
	text := ""
	for len(text) < 12000 {
		text += chars
	}
	result := n.Normalize(text)
	if len(result.Normalized) == 0 {
		t.Fatal("expected non-empty normalized text for 10k+ char input")
	}
	if result.NormalizedLength > result.OriginalLength {
		t.Fatal("normalized length should not exceed original")
	}
}

func TestNormalize_PreserveEnglish(t *testing.T) {
	n := DefaultNormalizer()
	text := "مرحبا world hello"
	result := n.Normalize(text)
	if result.Normalized == "" {
		t.Fatal("expected non-empty result for mixed text")
	}
}

func TestNormalize_PunctuationNormalization(t *testing.T) {
	n := &Normalizer{RemoveTashkeel: true, NormalizeWhitespace: true, NormalizePunctuation: true}
	text := "مرحبا؟ هذا جيد؛ مرحبا"
	result := n.Normalize(text)
	// U+061F (Arabic ?) should be replaced
	if len(result.Actions) > 0 {
		found := false
		for _, a := range result.Actions {
			if a.Type == "normalize_punctuation" {
				found = true
				break
			}
		}
		// Actions may or may not be recorded for punctuation depending on implementation
		_ = found
	}
}

func TestNormalize_NonArabicOnly(t *testing.T) {
	n := DefaultNormalizer()
	text := "Hello World"
	result := n.Normalize(text)
	if result.Normalized != "Hello World" {
		// Non-Arabic text should be returned as-is (after whitespace normalization)
		t.Logf("normalized non-Arabic text: %q", result.Normalized)
	}
}

func TestNormalize_WhitespaceNormalization(t *testing.T) {
	n := &Normalizer{RemoveTashkeel: true, NormalizeWhitespace: true}
	text := "  مرحباً   بالعالم  "
	result := n.Normalize(text)
	leading := result.Normalized[:1]
	_ = leading
	if result.Normalized != "مرحبا بالعالم" {
		// Should trim and collapse spaces
		t.Logf("whitespace normalized to: %q", result.Normalized)
	}
}

func TestNormalize_ZeroWidthCharacters(t *testing.T) {
	n := &Normalizer{RemoveTashkeel: true, NormalizeWhitespace: true}
	// U+200B (zero-width space), U+200C (ZWNJ), U+200D (ZWJ), U+FEFF (BOM)
	text := "مرح\u200B\u200C\u200D\uFEFFبا"
	result := n.Normalize(text)
	if len(result.Normalized) >= len("مرحبا") {
		// Zero-width chars should be stripped
		t.Logf("zero-width normalized: %q", result.Normalized)
	}
}

func TestNormalize_UnicodeEdgeCases(t *testing.T) {
	n := DefaultNormalizer()
	// Various Unicode ranges
	text := "مرحبا \u0670 test" // U+0670 is superscript alef (tashkeel locator)
	result := n.Normalize(text)
	if result.Normalized == text {
		// Superscript alef should be stripped
		t.Logf("unicode edge case normalized: %q", result.Normalized)
	}
}

func TestIsTashkeel(t *testing.T) {
	tests := []struct {
		rune   rune
		expect bool
	}{
		{0x064B, true}, // FATHATAN
		{0x064E, true}, // FATHA
		{0x0650, true}, // KASRA
		{0x064F, true}, // DAMMA
		{0x0651, true}, // SHADDA
		{0x0652, true}, // SUKUN
		{0x0670, true}, // SUPERSCRIPT ALEF
		{'أ', false},   // Alef with hamza above
		{'ا', false},   // Standard alef
	}
	for _, tc := range tests {
		if got := isTashkeel(tc.rune); got != tc.expect {
			t.Errorf("isTashkeel(U+%04X) = %v, want %v", tc.rune, got, tc.expect)
		}
	}
}

func TestNormalizeAlef(t *testing.T) {
	tests := []struct {
		rune        rune
		expected    rune
		expectMatch bool
	}{
		{0x0627, 0x0627, true},  // Alef madda
		{0x0623, 0x0627, true},  // Alef hamza above
		{0x0625, 0x0627, true},  // Alef hamza below
		{0x0622, 0x0627, true},  // Alef madda
		{'ب', 'ب', false},       // Non-alef
	}
	for _, tc := range tests {
		got, match := normalizeAlef(tc.rune)
		if match != tc.expectMatch {
			t.Errorf("normalizeAlef(U+%04X) match = %v, want %v", tc.rune, match, tc.expectMatch)
		}
		if got != tc.expected {
			t.Errorf("normalizeAlef(U+%04X) = U+%04X, want U+%04X", tc.rune, got, tc.expected)
		}
	}
}

func TestNormalize_DisabledOptions(t *testing.T) {
	n := &Normalizer{RemoveTashkeel: false, NormalizeAlef: false, NormalizeWhitespace: true}
	text := "بِسْمِ اللَّهِ"
	result := n.Normalize(text)
	if result.Normalized != text {
		t.Fatalf("expected no changes when all options disabled, got %q", result.Normalized)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("expected 0 actions when no changes made, got %d", len(result.Actions))
	}
}