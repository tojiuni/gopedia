package chunker

import (
	"fmt"
	"strings"
	"unicode"

	"gopedia/internal/phloem/types"
)

// ByFixedSizeChunker splits content into size-bounded chunks using semantic boundaries
// (newlines, sentence end, commas, pipes, brackets, etc.) so splits are less arbitrary than
// raw rune windows. Chunks do not overlap.
//
// Long-term: pair with AST/byte-offset parsing for zero-overlap, lossless structure and
// normalized (non-spurious) whitespace for viewer restore.
type ByFixedSizeChunker struct {
	MaxChars int // max characters per chunk; 0 = default 500
}

// Chunks ignores roots and splits content by semantic boundaries under MaxChars.
// Each chunk gets a generated SectionID and Path. SemanticL3Split is set so the sink can
// further split long clauses on commas/pipes with L3 parent chaining.
func (c ByFixedSizeChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	max := c.MaxChars
	if max <= 0 {
		max = 500
	}
	rs := []rune(strings.TrimSpace(content))
	if len(rs) == 0 {
		if strings.TrimSpace(content) == "" {
			return nil, nil
		}
		return []types.Chunk{{
			SectionID:       "s0",
			Path:            "",
			Text:            strings.TrimSpace(content),
			Level:           types.LevelL2,
			SemanticL3Split: true,
		}}, nil
	}

	var out []types.Chunk
	start := 0
	for start < len(rs) {
		limit := start + max
		if limit > len(rs) {
			limit = len(rs)
		}
		end := limit
		if limit < len(rs) {
			if br := findLastSemanticBreak(start, limit, rs); br > start {
				end = br
			} else {
				end = limit // hard cut at max if no semantic break
			}
		}
		if end <= start {
			end = start + 1
			if end > len(rs) {
				break
			}
		}
		text := strings.TrimSpace(string(rs[start:end]))
		if text != "" {
			idx := len(out)
			out = append(out, types.Chunk{
				SectionID:       fmtChunkID(idx),
				Path:            "",
				Text:            text,
				Level:           types.LevelL2,
				SemanticL3Split: true,
			})
		}
		start = end
		// Skip only pure whitespace between chunks (collapse spurious blank gaps).
		for start < len(rs) && unicode.IsSpace(rs[start]) {
			start++
		}
	}
	if len(out) == 0 && content != "" {
		out = append(out, types.Chunk{
			SectionID:       "s0",
			Path:            "",
			Text:            strings.TrimSpace(content),
			Level:           types.LevelL2,
			SemanticL3Split: true,
		})
	}
	return out, nil
}

// findLastSemanticBreak returns an exclusive end index in (start, limit] for a chunk, or start if none.
func findLastSemanticBreak(start, limit int, rs []rune) int {
	if limit > len(rs) {
		limit = len(rs)
	}
	for end := limit; end > start; end-- {
		prev := rs[end-1]
		if prev == '\n' {
			return end
		}
		if prev == '.' || prev == '!' || prev == '?' {
			if end == len(rs) || unicode.IsSpace(rs[end]) {
				return end
			}
		}
		if prev == ',' || prev == '|' || prev == ';' {
			return end
		}
		if prev == ')' || prev == '}' {
			return end
		}
		// Split before opening delimiters: previous chunk ends just before them.
		if prev == '(' || prev == '{' {
			if end-1 > start {
				return end - 1
			}
			continue
		}
		// Hyphen bullet after newline: "... \n- "
		if prev == '-' && end >= 2 && rs[end-2] == '\n' {
			return end
		}
		// Prefer breaking at whitespace if we have filled most of the window.
		if unicode.IsSpace(prev) && (end-start)*2 >= (limit-start) {
			return end
		}
	}
	return start
}

func fmtChunkID(i int) string {
	return fmt.Sprintf("s%d", i)
}
