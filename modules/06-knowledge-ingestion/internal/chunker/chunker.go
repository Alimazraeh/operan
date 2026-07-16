package chunker

import (
	"regexp"
	"strings"
	"unicode"
)

// Chunker splits text into chunks based on a configured strategy.
type Chunker interface {
	Chunk(text string, opts ChunkOptions) []Chunk
}

// ChunkOptions controls how text is split into chunks.
type ChunkOptions struct {
	Strategy   string
	ChunkSize  int // target size in characters
	ChunkOverlap int
}

// Chunk is a segment of extracted text with metadata.
type Chunk struct {
	Text     string
	Metadata map[string]any
	StartIdx int
	EndIdx   int
}

// AdaptiveChunker implements Chunker with adaptive strategy.
type AdaptiveChunker struct{}

// NewAdaptiveChunker creates a new AdaptiveChunker.
func NewAdaptiveChunker() *AdaptiveChunker {
	return &AdaptiveChunker{}
}

// Chunk implements Chunker.
func (c *AdaptiveChunker) Chunk(text string, opts ChunkOptions) []Chunk {
	switch opts.Strategy {
	case "fixed":
		return c.fixed(text, opts)
	case "by_heading":
		return c.byHeading(text, opts)
	case "by_paragraph":
		return c.byParagraph(text, opts)
	default:
		return c.adaptive(text, opts)
	}
}

// adaptive detects natural boundaries and fills chunks to target size.
func (c *AdaptiveChunker) adaptive(text string, opts ChunkOptions) []Chunk {
	// Detect heading boundaries first.
	sections := c.detectSections(text)
	if len(sections) > 0 {
		return c.chunkSections(sections, opts)
	}

	return c.fixed(text, opts)
}

// headingPattern matches markdown-style headings.
var headingPattern = regexp.MustCompile(`^#{1,6}\s+.+`)

// detectSections splits text at heading boundaries.
func (c *AdaptiveChunker) detectSections(text string) []section {
	var sections []section
	var current section
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if headingPattern.MatchString(strings.TrimSpace(line)) {
			if current.text != "" {
				sections = append(sections, current)
			}
			current = section{text: line + "\n", title: strings.TrimSpace(line)}
		} else if strings.TrimSpace(line) == "" {
			// Preserve blank lines within sections.
			current.text += line + "\n"
		} else if current.title != "" {
			current.text += line + "\n"
		} else {
			// Content without heading.
			if current.text != "" {
				sections = append(sections, current)
			}
			current = section{text: line + "\n", title: ""}
		}
	}
	if current.text != "" {
		sections = append(sections, current)
	}

	return sections
}

type section struct {
	title string
	text  string
}

func (c *AdaptiveChunker) chunkSections(sections []section, opts ChunkOptions) []Chunk {
	var chunks []Chunk
	buf := ""

	for _, s := range sections {
		buf += s.text
		if len(buf) >= opts.ChunkSize {
			chunks = append(chunks, Chunk{
				Text: strings.TrimSpace(buf),
				Metadata: map[string]any{
					"section_title": s.title,
				},
			})
			// Keep overlap portion.
			if len(buf) > opts.ChunkOverlap {
				buf = buf[len(buf)-opts.ChunkOverlap:]
			} else {
				buf = ""
			}
		}
	}
	if buf != "" {
		chunks = append(chunks, Chunk{
			Text: strings.TrimSpace(buf),
			Metadata: map[string]any{
				"section_title": sections[len(sections)-1].title,
			},
		})
	}

	return chunks
}

// fixed splits into equal-size chunks with overlap.
func (c *AdaptiveChunker) fixed(text string, opts ChunkOptions) []Chunk {
	var chunks []Chunk
	runes := []rune(text)
	total := len(runes)

	if total == 0 {
		return chunks
	}

	step := opts.ChunkSize
	if opts.ChunkOverlap > 0 {
		step = opts.ChunkSize - opts.ChunkOverlap
	}

	for start := 0; start < total; start += step {
		end := start + opts.ChunkSize
		if end > total {
			end = total
		}
		chunkText := string(runes[start:end])
		chunks = append(chunks, Chunk{
			Text:     chunkText,
			Metadata: map[string]any{"strategy": "fixed"},
			StartIdx: start,
			EndIdx:   end,
		})
	}

	return chunks
}

// byHeading splits at ## and ### headings.
func (c *AdaptiveChunker) byHeading(text string, opts ChunkOptions) []Chunk {
	var chunks []Chunk
	sectionPattern := regexp.MustCompile(`(?m)^#{2,4}\s+.+$`)
	locs := sectionPattern.FindAllStringIndex(text, -1)

	if len(locs) == 0 {
		return c.fixed(text, opts)
	}

	for i, loc := range locs {
		start := loc[0]
		var end int
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else {
			end = len(text)
		}
		section := text[start:end]
		heading := strings.TrimSpace(section[:strings.IndexByte(section, '\n')])
		chunks = append(chunks, Chunk{
			Text: strings.TrimSpace(section),
			Metadata: map[string]any{
				"strategy": "by_heading",
				"heading":  heading,
			},
			StartIdx: start,
			EndIdx:   end,
		})
	}

	return chunks
}

// byParagraph splits at double newlines, merges short ones.
func (c *AdaptiveChunker) byParagraph(text string, opts ChunkOptions) []Chunk {
	var chunks []Chunk
	parts := strings.Split(text, "\n\n")
	buf := ""
	bufLen := 0

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		partLen := runeCount(part)

		if bufLen+partLen > opts.ChunkSize && buf != "" {
			chunks = append(chunks, Chunk{
				Text:     buf,
				Metadata: map[string]any{"strategy": "by_paragraph"},
			})
			buf = part
			bufLen = partLen
		} else {
			if buf != "" {
				buf += "\n\n" + part
			} else {
				buf = part
			}
			bufLen += partLen + 2
		}
	}
	if buf != "" {
		chunks = append(chunks, Chunk{
			Text:     buf,
			Metadata: map[string]any{"strategy": "by_paragraph"},
		})
	}

	return chunks
}

// runeCount counts Unicode code points.
func runeCount(s string) int {
	count := 0
	for range s {
		count++
	}
	return count
}

// isWhitespace checks if a character is whitespace.
func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}