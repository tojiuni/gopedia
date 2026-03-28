// Package codesplitter splits code / fenced-block text into L3-sized fragments for Rhizome.
// Boilerplate implementation: strip markdown fences, emit one fragment per non-empty line.
// Future: language-specific splitting (Go/Python via AST) for domain/code ingestion.
package codesplitter

import "strings"

// SplitToL3 turns an L2 code chunk body into atomic lines/blocks for knowledge_l3.
// codeText may include markdown fences (```lang ... ```) as produced by the markdown chunker.
// lang is a hint (e.g. "mermaid", "go"); the baseline splitter ignores it except for future use.
func SplitToL3(codeText, lang string) []string {
	_ = lang // reserved for language-specific strategies
	s := strings.TrimSpace(codeText)
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	lines = stripMarkdownFences(lines)
	var out []string
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		out = append(out, ln)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stripMarkdownFences(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	start, end := 0, len(lines)
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		start = 1
	}
	if end > start && strings.HasPrefix(strings.TrimSpace(lines[end-1]), "```") {
		end--
	}
	if start >= end {
		return nil
	}
	return lines[start:end]
}
