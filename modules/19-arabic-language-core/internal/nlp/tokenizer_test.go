package nlp

import (
	"testing"
	"unicode/utf8"
)

func TestTokenizeWords_Simple(t *testing.T) {
	text := "مرحبا بالعالم"
	tokens := TokenizeWords(text)
	if len(tokens) < 2 {
		t.Errorf("expected at least 2 word tokens, got %d", len(tokens))
	}
	for _, tok := range tokens {
		if tok.Text == "" {
			t.Error("expected non-empty token text")
		}
		if tok.Start > tok.End {
			t.Errorf("start %d > end %d for token %q", tok.Start, tok.End, tok.Text)
		}
	}
}

func TestTokenizeWords_Empty(t *testing.T) {
	tokens := TokenizeWords("")
	if tokens != nil && len(tokens) != 0 {
		t.Errorf("expected nil or empty tokens, got %v", tokens)
	}
}

func TestTokenizeWords_WithSpaces(t *testing.T) {
	text := "السلام عليكم ورحمة الله"
	tokens := TokenizeWords(text)
	// 4 tokens: السلام، عليكم، ورحمة (connected)، الله
	expected := 4
	if len(tokens) != expected {
		t.Errorf("expected %d tokens, got %d: %v", expected, len(tokens), tokens)
	}
}

func TestTokenizeWords_MixedContent(t *testing.T) {
	text := "مرحبا world 123"
	tokens := TokenizeWords(text)
	if len(tokens) < 2 {
		t.Errorf("expected at least 2 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenizeChars_Simple(t *testing.T) {
	text := "أبج"
	tokens := TokenizeChars(text)
	if len(tokens) != 3 {
		t.Errorf("expected 3 char tokens, got %d", len(tokens))
	}
	for _, tok := range tokens {
		if tok.RuneName == "" {
			t.Error("expected non-empty rune name")
		}
	}
}

func TestTokenizeChars_Empty(t *testing.T) {
	tokens := TokenizeChars("")
	if tokens != nil && len(tokens) != 0 {
		t.Errorf("expected nil or empty, got %v", tokens)
	}
}

func TestTokenizeChars_ArabicChars(t *testing.T) {
	text := "مرحبا"
	tokens := TokenizeChars(text)
	for _, tok := range tokens {
		if tok.Position < 0 {
			t.Errorf("expected non-negative position, got %d", tok.Position)
		}
	}
	if len(tokens) != utf8.RuneCountInString(text) {
		t.Errorf("expected %d char tokens, got %d", utf8.RuneCountInString(text), len(tokens))
	}
}

func TestTokenizeChars_MixedScripts(t *testing.T) {
	text := "مرحبا hello 123"
	tokens := TokenizeChars(text)
	if len(tokens) == 0 {
		t.Fatal("expected non-empty token list")
	}
	foundArabic := false
	foundLatin := false
	for _, tok := range tokens {
		if tok.Rune >= 0x0600 && tok.Rune <= 0x06FF {
			foundArabic = true
		}
		if (tok.Rune >= 'a' && tok.Rune <= 'z') || (tok.Rune >= 'A' && tok.Rune <= 'Z') {
			foundLatin = true
		}
	}
	if !foundArabic || !foundLatin {
		t.Logf("mixed script tokens: arabic=%v latin=%v", foundArabic, foundLatin)
	}
}