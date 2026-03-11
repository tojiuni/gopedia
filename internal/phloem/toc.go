package phloem

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// TOCNode is one heading in the table of contents.
type TOCNode struct {
	Text     string    `json:"text"`
	Level    int       `json:"level"` // 1 = #, 2 = ##, etc.
	Children []TOCNode `json:"children,omitempty"`
}

// ParseTOC extracts a heading tree from markdown content.
// Headings are lines that start with # (1-6), then space, then the title.
func ParseTOC(content string) []TOCNode {
	lines := strings.Split(content, "\n")
	var roots []TOCNode
	var stack []*[]TOCNode // stack of child slices
	stack = append(stack, &roots)

	headingRe := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	for _, line := range lines {
		m := headingRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		level := len(m[1])
		text := strings.TrimSpace(m[2])

		node := TOCNode{Text: text, Level: level}

		// Pop stack until we're at the right parent level
		for len(stack) > 1 && len(*stack[len(stack)-1]) > 0 && (*stack[len(stack)-1])[len(*stack[len(stack)-1])-1].Level >= level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		*parent = append(*parent, node)
		stack = append(stack, &(*parent)[len(*parent)-1].Children)
	}

	return roots
}

// TOCToJSON returns the TOC tree as JSON string for RhizomeMessage.
func TOCToJSON(roots []TOCNode) (string, error) {
	b, err := json.Marshal(roots)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FlattenTOC returns all nodes in depth-first order with toc_path (e.g. "Root > Section > Sub").
func FlattenTOC(roots []TOCNode) []struct {
	Node      TOCNode
	Path      string
	SectionID string
} {
	var out []struct {
		Node      TOCNode
		Path      string
		SectionID string
	}
	var idx int
	var visit func(nodes []TOCNode, pathParts []string)
	visit = func(nodes []TOCNode, pathParts []string) {
		for _, n := range nodes {
			parts := append(append([]string{}, pathParts...), n.Text)
			path := strings.Join(parts, " > ")
			sectionID := fmt.Sprintf("s%d", idx)
			idx++
			out = append(out, struct {
				Node      TOCNode
				Path      string
				SectionID string
			}{Node: n, Path: path, SectionID: sectionID})
			visit(n.Children, parts)
		}
	}
	visit(roots, nil)
	return out
}
