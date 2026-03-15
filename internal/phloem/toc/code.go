package toc

import "gopedia/internal/phloem/types"

// CodeTOCParser extracts structure from source code (e.g. functions, classes).
// TODO: implement AST/symbol-based parsing per language.
type CodeTOCParser struct{}

// Parse returns a placeholder structure. Replace with real code parsing.
func (CodeTOCParser) Parse(content string) ([]types.TOCNode, error) {
	// Placeholder: single root node containing the whole file.
	return []types.TOCNode{{Text: "file", Level: 1}}, nil
}
