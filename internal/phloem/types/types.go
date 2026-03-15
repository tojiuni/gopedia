package types

// TOCNode is one heading or structure node in the table of contents.
type TOCNode struct {
	Text     string    `json:"text"`
	Level    int       `json:"level"` // 1 = #, 2 = ##, etc.
	Children []TOCNode `json:"children,omitempty"`
}

// FlatTOCItem is a TOC node with path and section ID (depth-first order).
type FlatTOCItem struct {
	Node      TOCNode
	Path      string
	SectionID string
}

// Chunk is the unit for embedding and Qdrant L2 (one per section/code block/page).
type Chunk struct {
	SectionID string
	Path      string
	Text      string
}
