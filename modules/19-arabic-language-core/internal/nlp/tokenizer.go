package nlp

import (
	"fmt"
	"strings"
)

// WordToken represents a word-level token with its position.
type WordToken struct {
	Text     string
	Start    int
	End      int
}

// CharToken represents a character-level token.
type CharToken struct {
	Rune     rune
	RuneName string
	Position int
}

// TokenizeWords splits Arabic text into word tokens.
// It handles connected Arabic letters and preserves word boundaries.
func TokenizeWords(text string) []WordToken {
	if text == "" {
		return nil
	}

	runes := []rune(text)
	var tokens []WordToken
	var current strings.Builder
	start := 0

	for i, r := range runes {
		if isArabicWordChar(r) {
			if current.Len() == 0 {
				start = i
			}
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, WordToken{
					Text:  current.String(),
					Start: start,
					End:   i,
				})
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, WordToken{
			Text:  current.String(),
			Start: start,
			End:   len(runes),
		})
	}
	return tokens
}

// TokenizeChars splits text into character-level tokens with metadata.
func TokenizeChars(text string) []CharToken {
	if text == "" {
		return nil
	}

	var tokens []CharToken
	for i, r := range text {
		name := runeName(r)
		tokens = append(tokens, CharToken{
			Rune:     r,
			RuneName: name,
			Position: i,
		})
	}
	return tokens
}

// runeName returns a descriptive name for a rune.
func runeName(r rune) string {
	if r >= 0x0600 && r <= 0x06FF {
		return fmt.Sprintf("ARABIC LETTER U+%04X", r)
	}
	return fmt.Sprintf("U+%04X", r)
}