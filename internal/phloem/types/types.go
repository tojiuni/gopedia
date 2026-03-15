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

// Level denotes the knowledge hierarchy: 1=L1 (doc), 2=L2 (section/header), 3=L3 (atomic).
const (
	LevelL1 = 1
	LevelL2 = 2
	LevelL3 = 3
)

// Chunk is the unit for embedding and storage. L2 = one per section/header; L3 = atomic sentence/block.
type Chunk struct {
	SectionID      string  // unique within doc (e.g. s0, s1)
	Path           string  // TOC path e.g. "Introduction > Goals"
	Text           string  // content for embedding
	Level          int     // LevelL2 or LevelL3; 0 treated as L2 for backward compat
	MachineID      int64   // optional; set when chunk has its own identity
	Version        int     // optional; for versioning/partial update
	QdrantID       string  // optional; reuse for unchanged L3
	ParentSectionID string // optional; L2 parent for L3 chunks
}
