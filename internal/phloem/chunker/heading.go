package chunker

import (
	"regexp"
	"strings"

	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

// ByHeadingChunker produces one chunk per TOC section (section body).
//
// Long-term: consider a markdown AST (e.g. goldmark) with byte offsets for zero-overlap chunks
// and spurious-whitespace normalization without losing real content.
type ByHeadingChunker struct{}

var mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// Chunks flattens the TOC and returns one Chunk per node.
// Each chunk is L2 and contains the section body (from the heading line up to the next heading
// at the same or higher level).
// If the document has text before the first ATX heading (frontmatter, preamble), it is preserved
// as SectionID "root" as the first chunk so ingestion does not drop that content.
func (ByHeadingChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	flat := toc.FlattenTOC(roots)
	intro := extractLeadingMarkdownIntro(content)

	var out []types.Chunk
	if strings.TrimSpace(intro) != "" {
		out = append(out, types.Chunk{
			SectionID: "root",
			Path:      "root",
			Text:      strings.TrimSpace(intro),
			Level:     types.LevelL2,
			Version:   1,
		})
	}

	sectionTextByID := extractMarkdownSections(content, flat)
	for i := range flat {
		out = append(out, types.Chunk{
			SectionID: flat[i].SectionID,
			Path:      flat[i].Path,
			Text:      sectionTextByID[flat[i].SectionID],
			Level:     types.LevelL2,
			Version:   1,
		})
	}
	return ExpandStructuredChunks(out), nil
}

// extractLeadingMarkdownIntro returns the substring before the first ATX heading line (# ...).
// If there is no heading, returns trimmed full content (entire doc is preamble-only).
func extractLeadingMarkdownIntro(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if mdHeadingRe.MatchString(strings.TrimSpace(line)) {
			if i == 0 {
				return ""
			}
			return strings.TrimSpace(strings.Join(lines[:i], "\n"))
		}
	}
	return strings.TrimSpace(content)
}

func extractMarkdownSections(content string, flat []types.FlatTOCItem) map[string]string {
	lines := strings.Split(content, "\n")

	type headingLoc struct {
		lineIdx int
		level   int
		text    string
	}

	// Find each heading in the order of flat TOC by scanning forward.
	locs := make([]headingLoc, 0, len(flat))
	searchFrom := 0
	for _, item := range flat {
		wantLevel := item.Node.Level
		wantText := strings.TrimSpace(item.Node.Text)
		found := -1
		for i := searchFrom; i < len(lines); i++ {
			m := mdHeadingRe.FindStringSubmatch(strings.TrimSpace(lines[i]))
			if m == nil {
				continue
			}
			level := len(m[1])
			text := strings.TrimSpace(m[2])
			if level == wantLevel && text == wantText {
				found = i
				locs = append(locs, headingLoc{lineIdx: i, level: level, text: text})
				searchFrom = i + 1
				break
			}
		}
		if found == -1 {
			// Fallback: keep deterministic ordering; extract empty text for this section.
			locs = append(locs, headingLoc{lineIdx: -1, level: wantLevel, text: wantText})
		}
	}

	out := make(map[string]string, len(flat))
	for idx, item := range flat {
		loc := locs[idx]
		if loc.lineIdx == -1 {
			out[item.SectionID] = ""
			continue
		}

		// Section ends at the next heading with level <= current level.
		end := len(lines)
		for j := loc.lineIdx + 1; j < len(lines); j++ {
			m := mdHeadingRe.FindStringSubmatch(strings.TrimSpace(lines[j]))
			if m == nil {
				continue
			}
			level := len(m[1])
			if level <= loc.level {
				end = j
				break
			}
		}

		section := strings.Join(lines[loc.lineIdx:end], "\n")
		out[item.SectionID] = strings.TrimSpace(section)
	}
	return out
}
