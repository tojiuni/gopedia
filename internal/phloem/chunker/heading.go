package chunker

import (
	"regexp"
	"strings"

	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

// ByHeadingChunker produces one chunk per TOC section (section body).
type ByHeadingChunker struct{}

var mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// Chunks flattens the TOC and returns one Chunk per node.
// Each chunk is L2 and contains the section body (from the heading line up to the next heading
// at the same or higher level).
func (ByHeadingChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	flat := toc.FlattenTOC(roots)
	sectionTextByID := extractMarkdownSections(content, flat)

	out := make([]types.Chunk, len(flat))
	for i := range flat {
		out[i] = types.Chunk{
			SectionID: flat[i].SectionID,
			Path:      flat[i].Path,
			Text:      sectionTextByID[flat[i].SectionID],
			Level:     types.LevelL2,
			Version:   1,
		}
	}
	return out, nil
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
