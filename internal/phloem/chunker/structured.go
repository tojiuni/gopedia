package chunker

import (
	"encoding/json"
	"regexp"
	"strings"

	"gopedia/internal/phloem/types"
)

// blockCounters assigns unique section_ids for derived L2 blocks within one document.
type blockCounters struct {
	ordered, table, code, image int
}

var (
	reOrderedItem = regexp.MustCompile(`^\s{0,3}(\d+)\.\s+(\S.*)$`)
	reMdImageLine = regexp.MustCompile(`^\s*!\[([^\]]*)\]\(([^)]+)\)\s*$`)
)

func isTableSeparatorRow(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.Contains(t, "|") {
		return false
	}
	t = strings.ReplaceAll(t, " ", "")
	if strings.HasPrefix(t, "|") {
		t = t[1:]
	}
	if strings.HasSuffix(t, "|") {
		t = t[:len(t)-1]
	}
	for _, part := range strings.Split(t, "|") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ok := true
		for _, r := range part {
			if r != '-' && r != ':' {
				ok = false
				break
			}
		}
		if !ok || len(part) < 3 {
			return false
		}
	}
	return true
}

func splitPipeRow(line string) []string {
	s := strings.TrimSpace(line)
	if strings.HasPrefix(s, "|") {
		s = s[1:]
	}
	if strings.HasSuffix(s, "|") {
		s = s[:len(s)-1]
	}
	parts := strings.Split(s, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// tryExtractGFMTable returns (lines consumed, fullText, meta) or 0,"",nil if not a table.
func tryExtractGFMTable(lines []string, start int) (int, string, map[string]any) {
	if start >= len(lines) || !strings.Contains(lines[start], "|") {
		return 0, "", nil
	}
	if start+1 >= len(lines) || !isTableSeparatorRow(lines[start+1]) {
		return 0, "", nil
	}
	header := lines[start]
	sep := lines[start+1]
	j := start + 2
	var body []string
	for j < len(lines) {
		row := lines[j]
		if strings.TrimSpace(row) == "" {
			break
		}
		if !strings.Contains(row, "|") {
			break
		}
		body = append(body, row)
		j++
	}
	cells := splitPipeRow(header)
	meta := map[string]any{
		"block_type":        types.SectionTypeTable,
		"parent_section_id": nil, // filled by caller
		"headers":           cells,
		"column_count":      len(cells),
		"separator_row":     sep,
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(sep)
	for _, r := range body {
		b.WriteByte('\n')
		b.WriteString(r)
	}
	return j - start, b.String(), meta
}

// splitHeadingLine returns the first ATX heading line (if any) and the remaining body.
func splitHeadingLine(text string) (heading, rest string) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return "", text
	}
	first := strings.TrimSpace(lines[0])
	if mdHeadingRe.MatchString(first) {
		if len(lines) == 1 {
			return first, ""
		}
		return first, strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	return "", text
}

// expandStructuredBlocks scans section body and emits derived L2 chunks (ordered/table/code/image).
// The returned slice always starts with the parent chunk with Text reduced to heading + remaining prose.
func expandStructuredBlocks(parent types.Chunk, ctr *blockCounters) []types.Chunk {
	if ctr == nil {
		ctr = &blockCounters{}
	}
	headingLine, body := splitHeadingLine(parent.Text)
	if strings.TrimSpace(body) == "" {
		out := parent
		if out.SectionType == "" {
			out.SectionType = types.SectionTypeHeading
		}
		if out.SourceMetadata == nil {
			out.SourceMetadata = map[string]any{"block_type": types.SectionTypeHeading}
		}
		return []types.Chunk{out}
	}

	lines := strings.Split(body, "\n")
	var prose strings.Builder

	writeProseLine := func(line string) {
		if prose.Len() > 0 {
			prose.WriteByte('\n')
		}
		prose.WriteString(line)
	}

	var derived []types.Chunk
	i := 0
	for i < len(lines) {
		line := lines[i]
		trim := strings.TrimSpace(line)

		if strings.HasPrefix(trim, "```") {
			ctr.code++
			sid := "c" + itoa(ctr.code)
			lang := strings.TrimSpace(trim[3:])
			var codeLines []string
			codeLines = append(codeLines, line)
			i++
			for i < len(lines) {
				codeLines = append(codeLines, lines[i])
				t := strings.TrimSpace(lines[i])
				if strings.HasPrefix(t, "```") {
					i++
					break
				}
				i++
			}
			codeText := strings.Join(codeLines, "\n")
			meta := map[string]any{
				"block_type":        types.SectionTypeCode,
				"parent_section_id": parent.SectionID,
				"language":          lang,
			}
			derived = append(derived, types.Chunk{
				SectionID:       sid,
				Path:            parent.Path + " > " + sid,
				Text:            codeText,
				Level:           types.LevelL2,
				Version:         1,
				ParentSectionID: parent.SectionID,
				SectionType:     types.SectionTypeCode,
				SourceMetadata:  meta,
			})
			continue
		}

		if m := reMdImageLine.FindStringSubmatch(trim); m != nil && len(trim) == len(strings.TrimSpace(lines[i])) {
			ctr.image++
			sid := "i" + itoa(ctr.image)
			meta := map[string]any{
				"block_type":        types.SectionTypeImage,
				"parent_section_id": parent.SectionID,
				"alt":               m[1],
				"url":               m[2],
			}
			derived = append(derived, types.Chunk{
				SectionID:       sid,
				Path:            parent.Path + " > " + sid,
				Text:            strings.TrimSpace(line),
				Level:           types.LevelL2,
				Version:         1,
				ParentSectionID: parent.SectionID,
				SectionType:     types.SectionTypeImage,
				SourceMetadata:  meta,
			})
			i++
			continue
		}

		if n, tbl, meta := tryExtractGFMTable(lines, i); n > 0 {
			meta["parent_section_id"] = parent.SectionID
			ctr.table++
			sid := "t" + itoa(ctr.table)
			derived = append(derived, types.Chunk{
				SectionID:       sid,
				Path:            parent.Path + " > " + sid,
				Text:            tbl,
				Level:           types.LevelL2,
				Version:         1,
				ParentSectionID: parent.SectionID,
				SectionType:     types.SectionTypeTable,
				SourceMetadata:  meta,
			})
			i += n
			continue
		}

		if reOrderedItem.FindStringSubmatch(line) != nil {
			// Collect all consecutive ordered list items (and their continuation lines)
			// into a single chunk so procedural sequences can be retrieved as a unit.
			var allLines []string
			for i < len(lines) {
				nl := lines[i]
				nt := strings.TrimSpace(nl)
				if nt == "" {
					break
				}
				if mdHeadingRe.MatchString(nt) {
					break
				}
				if strings.HasPrefix(nt, "```") {
					break
				}
				if reMdImageLine.MatchString(nt) {
					break
				}
				if strings.Contains(nl, "|") && i+1 < len(lines) && isTableSeparatorRow(lines[i+1]) {
					break
				}
				allLines = append(allLines, nl)
				i++
			}
			if len(allLines) > 0 {
				ctr.ordered++
				sid := "o" + itoa(ctr.ordered)
				itemText := strings.TrimSpace(strings.Join(allLines, "\n"))
				meta := map[string]any{
					"block_type":        types.SectionTypeOrdered,
					"parent_section_id": parent.SectionID,
				}
				derived = append(derived, types.Chunk{
					SectionID:       sid,
					Path:            parent.Path + " > " + sid,
					Text:            itemText,
					Level:           types.LevelL2,
					Version:         1,
					ParentSectionID: parent.SectionID,
					SectionType:     types.SectionTypeOrdered,
					SourceMetadata:  meta,
				})
			}
			continue
		}

		writeProseLine(line)
		i++
	}

	main := parent
	main.SectionType = types.SectionTypeHeading
	if main.SourceMetadata == nil {
		main.SourceMetadata = map[string]any{}
	}
	main.SourceMetadata["block_type"] = types.SectionTypeHeading

	var mainText strings.Builder
	if headingLine != "" {
		mainText.WriteString(headingLine)
	}
	ps := strings.TrimSpace(prose.String())
	if ps != "" {
		if mainText.Len() > 0 {
			mainText.WriteByte('\n')
		}
		mainText.WriteString(ps)
	}
	main.Text = strings.TrimSpace(mainText.String())

	out := make([]types.Chunk, 0, 1+len(derived))
	out = append(out, main)
	out = append(out, derived...)
	return out
}

func itoa(i int) string {
	if i <= 0 {
		return "0"
	}
	var b [16]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

// ExpandStructuredChunks runs expandStructuredBlocks on each input chunk in order (shared counters).
func ExpandStructuredChunks(chunks []types.Chunk) []types.Chunk {
	var ctr blockCounters
	var out []types.Chunk
	for _, c := range chunks {
		expanded := expandStructuredBlocks(c, &ctr)
		out = append(out, expanded...)
	}
	return out
}

// ChunkSourceMetadataJSON marshals chunk metadata for Postgres JSONB (nil -> "{}").
func ChunkSourceMetadataJSON(c types.Chunk) ([]byte, error) {
	if len(c.SourceMetadata) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(c.SourceMetadata)
}
