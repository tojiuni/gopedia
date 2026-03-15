package toc

import "gopedia/internal/phloem/types"

// PDFTOCParser extracts structure from PDF/OCR output (e.g. pages, blocks).
// TODO: implement page/block-based parsing when content is from OCR.
type PDFTOCParser struct{}

// Parse returns a placeholder structure. Replace with page/block parsing.
func (PDFTOCParser) Parse(content string) ([]types.TOCNode, error) {
	// Placeholder: single root node.
	return []types.TOCNode{{Text: "document", Level: 1}}, nil
}
