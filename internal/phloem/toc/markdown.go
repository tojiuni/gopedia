package toc

import (
	"regexp"
	"strings"

	"gopedia/internal/phloem/types"
)

// MarkdownTOCParser extracts a heading tree from markdown content (# to ######).
type MarkdownTOCParser struct{}

// Parse extracts headings from markdown. Headings are lines that start with # (1-6), then space, then the title.
func (MarkdownTOCParser) Parse(content string) ([]types.TOCNode, error) {
	lines := strings.Split(content, "\n")
	var roots []types.TOCNode
	var stack []*[]types.TOCNode
	stack = append(stack, &roots)

	headingRe := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	for _, line := range lines {
		m := headingRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		level := len(m[1])
		text := strings.TrimSpace(m[2])

		node := types.TOCNode{Text: text, Level: level}

		for len(stack) > 1 && len(*stack[len(stack)-1]) > 0 && (*stack[len(stack)-1])[len(*stack[len(stack)-1])-1].Level >= level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		*parent = append(*parent, node)
		stack = append(stack, &(*parent)[len(*parent)-1].Children)
	}

	return roots, nil
}
