package chunker

import (
	"fmt"
	"strings"

	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

// CodeChunker produces one L2 Chunk per top-level declaration (function/class)
// detected by the tree-sitter parser, with pre-computed L3Lines for 1-line = 1-L3 ingestion.
type CodeChunker struct {
	// Parser must be a *toc.CodeTOCParser so we can call ParseWithLines.
	Parser *toc.CodeTOCParser
}

// Chunks implements the Chunker interface.
// roots comes from CodeTOCParser.Parse() but we re-call ParseWithLines to get CodeLines.
func (c *CodeChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	_, lines, err := c.Parser.ParseWithLines(content)
	if err != nil {
		return nil, fmt.Errorf("CodeChunker ParseWithLines: %w", err)
	}
	return buildCodeChunks(content, lines), nil
}

// buildCodeChunks groups CodeLines into L2 chunks by top-level anchor boundaries.
// Each top-level anchor (parent_idx == -1, is_anchor == true) starts a new L2.
// Lines before the first top-level anchor form a "preamble" chunk (imports, module-level code).
func buildCodeChunks(content string, lines []types.CodeLine) []types.Chunk {
	if len(lines) == 0 {
		return []types.Chunk{{
			SectionID:   "f0",
			Path:        "file",
			Text:        content,
			Level:       types.LevelL2,
			SectionType: types.SectionTypeCode,
			SourceMetadata: map[string]any{
				"block_type": types.SectionTypeCode,
			},
		}}
	}

	// Find indices of top-level anchors (parent_idx == -1 && is_anchor == true).
	// These are function/class definitions at module scope.
	type boundary struct {
		lineIdx   int
		chunkName string
	}
	var topAnchors []boundary
	for i, ln := range lines {
		if ln.IsAnchor && ln.ParentIdx == -1 {
			topAnchors = append(topAnchors, boundary{lineIdx: i, chunkName: ln.Content})
		}
	}

	// No top-level anchors: whole file is one L2 chunk.
	if len(topAnchors) == 0 {
		return []types.Chunk{fileChunk(content, lines, "f1", "file")}
	}

	var chunks []types.Chunk

	// Preamble: lines before first top-level anchor.
	if topAnchors[0].lineIdx > 0 {
		preambleLines := lines[:topAnchors[0].lineIdx]
		if hasNonEmpty(preambleLines) {
			chunks = append(chunks, buildChunkWithOffset("pre", "preamble", content, preambleLines, 0))
		}
	}

	// One chunk per top-level anchor.
	for i, anchor := range topAnchors {
		start := anchor.lineIdx
		end := len(lines) // exclusive
		if i+1 < len(topAnchors) {
			end = topAnchors[i+1].lineIdx
		}
		chunkLines := lines[start:end]
		name := funcShortName(anchor.chunkName)
		sectionID := fmt.Sprintf("fn%d", i+1)
		chunks = append(chunks, buildChunkWithOffset(sectionID, name, content, chunkLines, start))
	}

	return chunks
}

// buildChunkWithOffset assembles one L2 Chunk from a slice of CodeLines.
// originalStart is the index of lines[0] within the original full lines slice,
// used to correctly re-index ParentIdx values to be chunk-relative.
func buildChunkWithOffset(sectionID, name, fullContent string, lines []types.CodeLine, originalStart int) types.Chunk {
	adjustedLines := make([]types.CodeLine, len(lines))
	copy(adjustedLines, lines)
	for ci := range adjustedLines {
		origParentIdx := adjustedLines[ci].ParentIdx
		if origParentIdx < 0 {
			adjustedLines[ci].ParentIdx = -1
			continue
		}
		// origParentIdx is absolute; convert to chunk-relative.
		relIdx := origParentIdx - originalStart
		if relIdx >= 0 && relIdx < len(lines) {
			adjustedLines[ci].ParentIdx = relIdx
		} else {
			adjustedLines[ci].ParentIdx = -1
		}
	}

	srcLines := strings.Split(fullContent, "\n")
	var textLines []string
	for _, cl := range lines {
		idx := cl.LineNum - 1 // convert to 0-based
		if idx >= 0 && idx < len(srcLines) {
			textLines = append(textLines, srcLines[idx])
		}
	}

	return types.Chunk{
		SectionID:   sectionID,
		Path:        name,
		Text:        strings.Join(textLines, "\n"),
		Level:       types.LevelL2,
		SectionType: types.SectionTypeCode,
		SourceMetadata: map[string]any{
			"block_type": types.SectionTypeCode,
		},
		L3Lines: adjustedLines,
	}
}

func fileChunk(content string, lines []types.CodeLine, sectionID, name string) types.Chunk {
	return types.Chunk{
		SectionID:   sectionID,
		Path:        name,
		Text:        content,
		Level:       types.LevelL2,
		SectionType: types.SectionTypeCode,
		SourceMetadata: map[string]any{
			"block_type": types.SectionTypeCode,
		},
		L3Lines: lines,
	}
}

// funcShortName extracts the identifier from a function/class declaration line.
// "def _pg_connect():" -> "_pg_connect"
// "func Register(s *fuego.Server)..." -> "Register"
func funcShortName(sig string) string {
	sig = strings.TrimSpace(sig)
	for _, prefix := range []string{"async def ", "def ", "func ", "class ", "type ", "const "} {
		if strings.HasPrefix(sig, prefix) {
			rest := strings.TrimPrefix(sig, prefix)
			if idx := strings.IndexAny(rest, "(:[ \t"); idx > 0 {
				return strings.TrimSpace(rest[:idx])
			}
			if f := strings.Fields(rest); len(f) > 0 {
				return f[0]
			}
		}
	}
	// Fallback: first word
	if f := strings.Fields(sig); len(f) > 0 {
		return f[0]
	}
	return sig
}

func hasNonEmpty(lines []types.CodeLine) bool {
	for _, l := range lines {
		if strings.TrimSpace(l.Content) != "" {
			return true
		}
	}
	return false
}
