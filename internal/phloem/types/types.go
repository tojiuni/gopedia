package types

// SectionType values for structured L2 chunks (Qdrant payload + knowledge_l2.source_metadata).
const (
	SectionTypeHeading = "heading"
	SectionTypeOrdered = "ordered"
	SectionTypeTable   = "table"
	SectionTypeCode    = "code"
	SectionTypeImage   = "image"
)

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

// CodeLine represents one source code line as an L3 unit.
// Produced by the tree-sitter parser (flows/code_parser) and consumed by DefaultSink.
// When Chunk.L3Lines is non-nil, DefaultSink uses these directly instead of sentence splitting.
type CodeLine struct {
	LineNum      int    // 1-based source file line number (used as sort_order base)
	Content      string // exact source line content; empty string for blank lines
	NodeType     string // tree-sitter node type: "function_definition", "import_statement", etc.
	IsAnchor     bool   // true = gets a Qdrant embedding vector + eligible as parent_id target
	IsBlockStart bool   // true = head of a multi-line compound expression (e.g. "return foo(")
	ParentIdx    int    // index into the containing Chunk.L3Lines slice; -1 = no parent within chunk
}

// Chunk is the unit for embedding and storage. L2 = one per section/header; L3 = atomic sentence/block.
type Chunk struct {
	SectionID       string // unique within doc (e.g. s0, s1, o1, t1)
	Path            string // TOC path e.g. "Introduction > Goals"
	Text            string // content for embedding
	Level           int    // LevelL2 or LevelL3; 0 treated as L2 for backward compat
	MachineID       int64  // optional; set when chunk has its own identity
	Version         int    // optional; for versioning/partial update
	QdrantID        string // optional; reuse for unchanged L3
	ParentSectionID string // optional; logical parent section_id (s*) for derived L2 (o/t/c/i)
	// SectionType: heading (default), ordered, table, code, image — for payload + PG JSON.
	SectionType string `json:"section_type,omitempty"`
	// SourceMetadata: persisted to knowledge_l2.source_metadata (e.g. table headers, code lang).
	SourceMetadata map[string]any `json:"source_metadata,omitempty"`
	// SemanticL3Split: when true, DefaultSink splits each sentence further on commas, pipes, etc.
	// so the first fragment is parent L3 and following fragments chain under it (same l2_id).
	SemanticL3Split bool `json:"semantic_l3_split,omitempty"`
	// L3Lines holds pre-computed per-line L3 data for the code domain.
	// When non-nil, DefaultSink skips sentence splitting and inserts these lines directly.
	// Always nil for Markdown chunks (backward-compatible).
	L3Lines []CodeLine `json:"l3_lines,omitempty"`
}
