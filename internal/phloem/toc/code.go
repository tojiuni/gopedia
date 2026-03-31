package toc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"gopedia/internal/phloem/types"
)

// codeParserTOCNode is the JSON shape returned by flows/code_parser/cli.py.
type codeParserTOCNode struct {
	Text      string `json:"text"`
	Level     int    `json:"level"`
	NodeType  string `json:"node_type"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// codeParserLine is the JSON shape for each source line from the parser.
type codeParserLine struct {
	LineNum      int    `json:"line_num"`
	Content      string `json:"content"`
	NodeType     string `json:"node_type"`
	IsAnchor     bool   `json:"is_anchor"`
	IsBlockStart bool   `json:"is_block_start"`
	ParentIdx    int    `json:"parent_idx"`
}

// codeParserResult is the top-level JSON output from flows/code_parser/cli.py.
type codeParserResult struct {
	TOC   []codeParserTOCNode `json:"toc"`
	Lines []codeParserLine    `json:"lines"`
}

// CodeTOCParser calls the Python tree-sitter CLI to parse source code structure.
// It implements TOCParser and also exposes ParseWithLines for the code chunker.
type CodeTOCParser struct {
	// Lang is the source language: "python", "go", "typescript".
	Lang string
	// RepoRoot is the working directory for the Python subprocess (repo root).
	// Defaults to GOPEDIA_REPO_ROOT env var.
	RepoRoot string
	// PythonBin is the Python executable. Defaults to GOPEDIA_PYTHON or "python3".
	PythonBin string
}

// Parse implements TOCParser. Returns top-level declaration nodes.
func (p *CodeTOCParser) Parse(content string) ([]types.TOCNode, error) {
	nodes, _, err := p.ParseWithLines(content)
	return nodes, err
}

// ParseWithLines calls the Python parser and returns both TOC nodes and per-line metadata.
// The Chunker uses the returned CodeLine slice to build L3Lines.
func (p *CodeTOCParser) ParseWithLines(content string) ([]types.TOCNode, []types.CodeLine, error) {
	py := p.pythonBin()
	root := p.repoRoot()
	lang := p.Lang
	if lang == "" {
		lang = "python"
	}

	cmd := exec.Command(py, "-m", "flows.code_parser.cli", "parse", "--lang", lang)
	cmd.Dir = root
	cmd.Stdin = bytes.NewBufferString(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("code_parser cli (lang=%s): %w; stderr: %s", lang, err, stderr.String())
	}

	var result codeParserResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, nil, fmt.Errorf("code_parser json unmarshal: %w", err)
	}

	tocNodes := make([]types.TOCNode, 0, len(result.TOC))
	for _, n := range result.TOC {
		tocNodes = append(tocNodes, types.TOCNode{
			Text:  n.Text,
			Level: n.Level,
		})
	}

	codeLines := make([]types.CodeLine, 0, len(result.Lines))
	for _, l := range result.Lines {
		codeLines = append(codeLines, types.CodeLine{
			LineNum:      l.LineNum,
			Content:      l.Content,
			NodeType:     l.NodeType,
			IsAnchor:     l.IsAnchor,
			IsBlockStart: l.IsBlockStart,
			ParentIdx:    l.ParentIdx,
		})
	}

	return tocNodes, codeLines, nil
}

func (p *CodeTOCParser) pythonBin() string {
	if p.PythonBin != "" {
		return p.PythonBin
	}
	if v := os.Getenv("GOPEDIA_PYTHON"); v != "" {
		return v
	}
	return "python3"
}

func (p *CodeTOCParser) repoRoot() string {
	if p.RepoRoot != "" {
		return p.RepoRoot
	}
	if v := os.Getenv("GOPEDIA_REPO_ROOT"); v != "" {
		return v
	}
	return "."
}
