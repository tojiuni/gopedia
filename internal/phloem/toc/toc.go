package toc

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopedia/internal/phloem/types"
)

// TOCParser extracts a structure (e.g. heading tree) from content.
type TOCParser interface {
	Parse(content string) ([]types.TOCNode, error)
}

// FlattenTOC returns all nodes in depth-first order with toc_path (e.g. "Root > Section > Sub").
func FlattenTOC(roots []types.TOCNode) []types.FlatTOCItem {
	var out []types.FlatTOCItem
	var idx int
	var visit func(nodes []types.TOCNode, pathParts []string)
	visit = func(nodes []types.TOCNode, pathParts []string) {
		for _, n := range nodes {
			parts := append(append([]string{}, pathParts...), n.Text)
			path := strings.Join(parts, " > ")
			sectionID := fmt.Sprintf("s%d", idx)
			idx++
			out = append(out, types.FlatTOCItem{Node: n, Path: path, SectionID: sectionID})
			visit(n.Children, parts)
		}
	}
	visit(roots, nil)
	return out
}

// TOCToJSON returns the TOC tree as JSON string for RhizomeMessage.
func TOCToJSON(roots []types.TOCNode) (string, error) {
	b, err := json.Marshal(roots)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
