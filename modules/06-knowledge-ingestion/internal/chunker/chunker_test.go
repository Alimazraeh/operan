package chunker

import (
	"strings"
	"testing"
)

func newChunker() *AdaptiveChunker {
	return NewAdaptiveChunker()
}

// === Adaptive Strategy ===

func TestChunker_Adaptive_Default(t *testing.T) {
	c := newChunker()
	text := "This is a test document with multiple sentences. " +
		"It should be split into chunks based on natural boundaries. " +
		"Each chunk should be approximately 256 characters with some overlap."
	opts := ChunkOptions{Strategy: "adaptive", ChunkSize: 256, ChunkOverlap: 30}
	chunks := c.Chunk(text, opts)

	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk, got %d", len(chunks))
	}

	// Verify overlap between consecutive chunks.
	for i := 1; i < len(chunks); i++ {
		prevEnd := chunks[i-1].EndIdx
		currStart := chunks[i].StartIdx
		// Overlap should be positive.
		if prevEnd < currStart {
			t.Errorf("chunks %d and %d have no overlap (prevEnd=%d, currStart=%d)", i-1, i, prevEnd, currStart)
		}
	}
}

func TestChunker_Adaptive_WithHeadings(t *testing.T) {
	c := newChunker()
	text := "## Introduction\nThis is the intro section with several sentences of text content that should span multiple lines.\n\n## Methods\nThe methods section describes how we did things in detail.\n\n## Results\nHere are the results we found."
	opts := ChunkOptions{Strategy: "adaptive", ChunkSize: 256, ChunkOverlap: 20}
	chunks := c.Chunk(text, opts)

	// Adaptive may produce 1 or more chunks depending on content.
	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk for heading-separated text, got %d", len(chunks))
	}
}

// === Fixed Strategy ===

func TestChunker_Fixed(t *testing.T) {
	c := newChunker()
	text := strings.Repeat("A ", 200) // ~400 chars
	opts := ChunkOptions{Strategy: "fixed", ChunkSize: 100, ChunkOverlap: 10}
	chunks := c.Chunk(text, opts)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for 400 chars with 100-char size, got %d", len(chunks))
	}

	// Each chunk should be approximately ChunkSize.
	for i, ch := range chunks {
		maxSize := opts.ChunkSize + opts.ChunkOverlap
		if len(ch.Text) > maxSize {
			t.Errorf("chunk %d text too long: %d > %d", i, len(ch.Text), maxSize)
		}
	}
}

func TestChunker_Fixed_SingleChunk(t *testing.T) {
	c := newChunker()
	text := "Short text."
	opts := ChunkOptions{Strategy: "fixed", ChunkSize: 100, ChunkOverlap: 10}
	chunks := c.Chunk(text, opts)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
}

// === By Heading Strategy ===

func TestChunker_ByHeading(t *testing.T) {
	c := newChunker()
	text := "## Section One\nSome content here.\n\n## Section Two\nMore content here.\n\n## Section Three\nEven more content."
	opts := ChunkOptions{Strategy: "by_heading", ChunkSize: 512, ChunkOverlap: 0}
	chunks := c.Chunk(text, opts)

	// Should produce 3 chunks (one per heading).
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks for 3 headings, got %d", len(chunks))
	}

	for i, ch := range chunks {
		expected := []string{"## Section One", "## Section Two", "## Section Three"}
		if !strings.Contains(ch.Text, expected[i]) {
			t.Errorf("chunk %d missing expected heading: %q (text=%q)", i, expected[i], ch.Text)
		}
	}
}

// === By Paragraph Strategy ===

func TestChunker_ByParagraph(t *testing.T) {
	c := newChunker()
	text := "First paragraph here.\n\nSecond paragraph here.\n\nThird paragraph here.\n\nFourth paragraph here."
	opts := ChunkOptions{Strategy: "by_paragraph", ChunkSize: 100, ChunkOverlap: 10}
	chunks := c.Chunk(text, opts)

	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk, got %d", len(chunks))
	}

	// Verify paragraphs are split.
	paragraphCount := strings.Count(text, "\n\n")
	if len(chunks) < paragraphCount/2 {
		t.Logf("got %d chunks for %d paragraphs (may merge short ones)", len(chunks), paragraphCount)
	}
}

// === Very Long Text ===

func TestChunker_VeryLongText(t *testing.T) {
	c := newChunker()
	text := strings.Repeat("Lorem ipsum dolor sit amet. ", 100)
	opts := ChunkOptions{Strategy: "fixed", ChunkSize: 128, ChunkOverlap: 20}
	chunks := c.Chunk(text, opts)

	if len(chunks) < 5 {
		t.Fatalf("expected many chunks for very long text, got %d", len(chunks))
	}

	// Verify first and last chunks have content.
	if len(chunks[0].Text) == 0 {
		t.Error("first chunk is empty")
	}
	if len(chunks[len(chunks)-1].Text) == 0 {
		t.Error("last chunk is empty")
	}
}

// === Short Text ===

func TestChunker_ShortText(t *testing.T) {
	c := newChunker()
	text := "Hi."
	opts := ChunkOptions{Strategy: "adaptive", ChunkSize: 256, ChunkOverlap: 30}
	chunks := c.Chunk(text, opts)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
	if chunks[0].Text != text {
		t.Errorf("expected exact text, got %q", chunks[0].Text)
	}
}

// === Unknown Strategy ===

func TestChunker_UnknownStrategy(t *testing.T) {
	c := newChunker()
	text := "Some text here."
	opts := ChunkOptions{Strategy: "unknown", ChunkSize: 100, ChunkOverlap: 10}
	chunks := c.Chunk(text, opts)

	// Should fall back to fixed strategy.
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk even with unknown strategy")
	}
}

// === Chunk Index Tracking ===

func TestChunker_IndexTracking(t *testing.T) {
	c := newChunker()
	text := "First part. Second part. Third part."
	opts := ChunkOptions{Strategy: "fixed", ChunkSize: 15, ChunkOverlap: 3}
	chunks := c.Chunk(text, opts)

	step := opts.ChunkSize - opts.ChunkOverlap
	for i, ch := range chunks {
		if ch.StartIdx != i*step {
			t.Errorf("chunk %d StartIdx=%d, expected %d", i, ch.StartIdx, i*step)
		}
		if ch.EndIdx < ch.StartIdx {
			t.Errorf("chunk %d EndIdx < StartIdx", i)
		}
	}
}