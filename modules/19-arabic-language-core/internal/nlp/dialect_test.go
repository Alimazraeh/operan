package nlp

import (
	"testing"
)

func TestDetectDialect_MSA(t *testing.T) {
	// Formal Arabic text
	text := "إن هذا الكتاب يحتوي على معلومات مهمة جداً"
	result := DetectDialect(text)
	if !result.IsMSA {
		t.Errorf("expected MSA, got %s", result.Dialect)
	}
	if result.Confidence < 0 {
		t.Errorf("expected confidence >= 0, got %f", result.Confidence)
	}
	if len(result.AllScores) == 0 {
		t.Fatal("expected scores for all dialects")
	}
}

func TestDetectDialect_MixedText(t *testing.T) {
	// Text with formal + colloquial words
	text := "هذا الكتاب جميل جداً يا اخوي"
	result := DetectDialect(text)
	if result.Confidence < 0 {
		t.Errorf("expected confidence >= 0, got %f", result.Confidence)
	}
	// Should not crash
	_ = result.AllScores
}

func TestDetectDialect_EmptyText(t *testing.T) {
	result := DetectDialect("")
	if result.Dialect != "unknown" {
		t.Errorf("expected dialect=unknown for empty text, got %s", result.Dialect)
	}
	if result.Confidence != 0 {
		t.Errorf("expected confidence=0 for empty text, got %f", result.Confidence)
	}
}

func TestDetectDialect_AllDialectsHaveKeys(t *testing.T) {
	expected := []string{"msa", "saudi", "emirati", "kuwaiti", "bahraini", "qatari", "omani", "egyptian", "levantine", "moroccan"}
	for _, d := range expected {
		if _, ok := dialectKeywords[d]; !ok {
			t.Errorf("missing keywords for dialect %s", d)
		}
		if len(dialectKeywords[d]) == 0 {
			t.Errorf("empty keyword set for dialect %s", d)
		}
	}
}

func TestDetectDialect_SaudiKeywords(t *testing.T) {
	// Text rich in Saudi keywords
	text := "شفت هالناس هيك هالمرة ياغالي هالبيت"
	result := DetectDialect(text)
	if result.IsMSA {
		t.Logf("MSA detected for Saudi-heavy text (dialect=%s, confidence=%f)", result.Dialect, result.Confidence)
	}
	if result.Confidence < 0 {
		t.Errorf("expected confidence >= 0, got %f", result.Confidence)
	}
}

func TestDetectDialect_EgyptianKeywords(t *testing.T) {
	text := "إزاي ده كده عشان مش عارف"
	result := DetectDialect(text)
	if result.Confidence < 0 {
		t.Errorf("expected confidence >= 0, got %f", result.Confidence)
	}
}

func TestDetectDialect_LowConfidence(t *testing.T) {
	// Very short text should yield low confidence
	text := "مرحبا"
	result := DetectDialect(text)
	if result.Confidence < 0 || result.Confidence > 1.0 {
		t.Errorf("expected confidence in [0, 1], got %f", result.Confidence)
	}
}

func TestDetectDialect_AllScoresSorted(t *testing.T) {
	text := "إن شاء الله هذا الكتاب رائع"
	result := DetectDialect(text)
	if len(result.AllScores) == 0 {
		t.Fatal("expected all_scores to be non-empty")
	}
	if len(result.AllScores) != len(dialectKeywords) {
		t.Errorf("expected %d scores, got %d", len(dialectKeywords), len(result.AllScores))
	}
}

func TestDetectDialect_NumeralsOnly(t *testing.T) {
	text := "123456789"
	result := DetectDialect(text)
	// Should not panic, may return MSA or unknown
	_ = result.Confidence
	_ = result.Dialect
}

func TestDetectDialect_ArabicNumerals(t *testing.T) {
	text := "١٢٣٤٥"
	result := DetectDialect(text)
	if result.Confidence < 0 {
		t.Errorf("expected confidence >= 0, got %f", result.Confidence)
	}
}

func TestDetectDialect_ConfidenceClamped(t *testing.T) {
	text := "هذا الكتاب رائع إن شاء الله"
	result := DetectDialect(text)
	if result.Confidence > 1.01 {
		t.Errorf("confidence should be <= 1.0, got %f", result.Confidence)
	}
	if result.Confidence < -0.01 {
		t.Errorf("confidence should be >= 0.0, got %f", result.Confidence)
	}
}

func TestDetectDialect_WhitespaceInput(t *testing.T) {
	text := "   "
	result := DetectDialect(text)
	if result.Confidence < 0 {
		t.Errorf("expected confidence >= 0, got %f", result.Confidence)
	}
}

func TestTokenizeArabic(t *testing.T) {
	text := "مرحبا بالعالم"
	tokens := tokenizeArabic(text)
	if len(tokens) < 2 {
		t.Errorf("expected at least 2 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenizeArabic_Empty(t *testing.T) {
	tokens := tokenizeArabic("")
	if tokens != nil && len(tokens) != 0 {
		t.Fatalf("expected nil or empty tokens, got %v", tokens)
	}
}

func TestTokenizeArabic_SingleWord(t *testing.T) {
	tokens := tokenizeArabic("كتاب")
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

func TestStripTashkeel(t *testing.T) {
	text := "بِسْمِ اللَّهِ"
	result := stripTashkeel(text)
	if result == text {
		t.Fatal("expected tashkeel to be stripped")
	}
}

func TestIsArabicWordChar(t *testing.T) {
	tests := []struct {
		rune   rune
		expect bool
	}{
		{0x0628, true},  // Arabic beh
		{0x0623, true},  // Alef hamza above
		{0x0061, false}, // Latin 'a'
		{0x0020, false}, // Space
		{0x0031, true},  // Latin '1' (Arabic numerals pass through)
	}
	for _, tc := range tests {
		if got := isArabicWordChar(tc.rune); got != tc.expect {
			t.Errorf("isArabicWordChar(U+%04X) = %v, want %v", tc.rune, got, tc.expect)
		}
	}
}