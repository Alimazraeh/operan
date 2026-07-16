package nlp

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Arabic normalizer that handles canonical text form.
type Normalizer struct {
	// Options control which normalizations are applied.
	RemoveTashkeel      bool
	NormalizeAlef       bool
	NormalizeHamza      bool
	NormalizeNoalif     bool
	NormalizeWhitespace bool
	NormalizePunctuation bool
	ConvertNumerals     bool
}

// NormalizationAction records what was changed during normalization.
type NormalizationAction struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Position    *int   `json:"position,omitempty"`
}

// NormalizeResult holds the output of text normalization.
type NormalizeResult struct {
	Normalized       string              `json:"normalized"`
	Actions          []NormalizationAction `json:"actions"`
	OriginalLength   int                 `json:"original_length"`
	NormalizedLength int                 `json:"normalized_length"`
}

// DefaultNormalizer returns a Normalizer with all options enabled.
func DefaultNormalizer() *Normalizer {
	return &Normalizer{
		RemoveTashkeel:      true,
		NormalizeAlef:       true,
		NormalizeHamza:      true,
		NormalizeNoalif:     true,
		NormalizeWhitespace: true,
		NormalizePunctuation: true,
	}
}

// Normalize applies all configured normalizations to the input text.
func (n *Normalizer) Normalize(text string) NormalizeResult {
	result := NormalizeResult{
		OriginalLength:  utf8.RuneCountInString(text),
		Actions:         make([]NormalizationAction, 0),
	}

	if text == "" {
		result.Normalized = ""
		result.NormalizedLength = 0
		return result
	}

	runes := []rune(text)
	actions := make([]NormalizationAction, 0)
	changed := runes // work in-place

	i := 0
	for i < len(changed) {
		r := changed[i]

		if n.RemoveTashkeel && isTashkeel(r) {
			pos := i
			actions = append(actions, NormalizationAction{
				Type:        "remove_tashkeel",
				Description: "removed diacritic mark",
				Position:    &pos,
			})
			changed = append(changed[:i], changed[i+1:]...)
			// Don't increment i — next rune shifted into this position
			continue
		}

		if n.NormalizeAlef {
			if normalized, ok := normalizeAlef(r); ok {
				changed[i] = normalized
				actions = append(actions, NormalizationAction{
					Type:        "normalize_alef",
					Description: "unified alef variant to standard alef",
					Position:    &i,
				})
			}
		}

		if n.NormalizeNoalif {
			if changed[i] == 'ى' {
				changed[i] = 'ى'
			}
		}

		i++
	}

	// Rebuild string after tashkeel removal (indices may have shifted)
	if len(actions) > 0 && n.RemoveTashkeel {
		// Re-apply alef/hamza on cleaned string
		cleaned := string(changed)
		actions2 := n.applyCharLevel(cleaned)
		actions = append(actions, actions2...)
		result.Normalized = cleaned
	} else {
		result.Normalized = string(changed)
	}

	// Punctuation normalization
	if n.NormalizePunctuation {
		result.Normalized = n.normalizePunctuation(result.Normalized, &actions)
	}

	// Whitespace normalization
	if n.NormalizeWhitespace {
		result.Normalized = n.normalizeWhitespace(result.Normalized, &actions)
	}

	result.Actions = actions
	result.NormalizedLength = utf8.RuneCountInString(result.Normalized)
	return result
}

// isTashkeel checks if a rune is a diacritical mark (tashkeel).
func isTashkeel(r rune) bool {
	// U+064B–U+065F covers fatha, kasra, damma, shadda, sukun, tanween, etc.
	return (r >= 0x064B && r <= 0x065F) ||
		r == 0x0670 // U+0670: superscript alef (tashkeel locator)
}

// normalizeAlef returns the unified alef rune and true if the input was an alef variant.
func normalizeAlef(r rune) (rune, bool) {
	switch r {
	case 0x0627, 0x0623, 0x0625, 0x0622:
		// All alef variants → standard alef (U+0627)
		return 0x0627, true
	}
	return r, false
}

// applyCharLevel applies character-level normalization on the cleaned text.
func (n *Normalizer) applyCharLevel(text string) []NormalizationAction {
	actions := make([]NormalizationAction, 0)
	runes := []rune(text)

	for i, r := range runes {
		if n.NormalizeHamza && isHamzaVariant(r) {
			runes[i] = 0x0627 // Normalize to standard alef
			pos := i
			actions = append(actions, NormalizationAction{
				Type:        "normalize_hamza",
				Description: "normalized hamza variant to standard alef",
				Position:    &pos,
			})
		}
	}
	return actions
}

// normalizePunctuation converts Arabic punctuation to canonical forms.
func (n *Normalizer) normalizePunctuation(text string, actions *[]NormalizationAction) string {
	r := []rune(text)
	for i, c := range r {
		switch c {
		case 0x060C: // Arabic comma
			r[i] = ','
		case 0x061B: // Arabic semicolon
			r[i] = ';'
		case 0x061F: // Arabic question mark
			r[i] = '?'
		case 0x00AB: // « left angle quote
			r[i] = '"'
		case 0x00BB: // » right angle quote
			r[i] = '"'
		}
	}
	return string(r)
}

// normalizeWhitespace removes zero-width characters and normalizes spaces.
func (n *Normalizer) normalizeWhitespace(text string, actions *[]NormalizationAction) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch r {
		case 0x200B: // Zero-width space
		case 0x200C: // Zero-width non-joiner
		case 0x200D: // Zero-width joiner
		case 0xFEFF: // BOM / zero-width no-break space
		case 0x00A0: // Non-breaking space
			b.WriteRune(' ')
		default:
			if unicode.IsSpace(r) {
				b.WriteRune(' ')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// isHamzaVariant checks if a rune is a hamza on a seat (alef, waive, ya, etc.).
func isHamzaVariant(r rune) bool {
	return r == 0x0623 || r == 0x0625 || r == 0x0622 ||
		r == 0x0624 || r == 0x0621
}