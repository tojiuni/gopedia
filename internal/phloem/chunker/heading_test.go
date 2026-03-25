package chunker

import (
	"strings"
	"testing"

	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

func TestByHeadingChunkerExtractsSectionBodies(t *testing.T) {
	md := strings.TrimSpace(`
---
title: Sample
---

# Introduction

Intro paragraph.

## Goals

We want to verify keyword search and section context.

## Implementation

Root sends markdown to Phloem; Phloem writes to PostgreSQL, Qdrant, and TypeDB sync writes document and sections.

# Appendix

End.
`)

	roots, err := (toc.MarkdownTOCParser{}).Parse(md)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	chunks, err := (ByHeadingChunker{}).Chunks(md, roots)
	if err != nil {
		t.Fatalf("chunks: %v", err)
	}
	if len(chunks) < 4 {
		t.Fatalf("expected >=4 chunks, got %d", len(chunks))
	}

	// Introduction should include its own body and subheadings until next #.
	intro := chunks[0]
	if intro.Level != types.LevelL2 {
		t.Fatalf("expected L2 chunk, got level=%d", intro.Level)
	}
	if !strings.Contains(intro.Text, "# Introduction") {
		t.Fatalf("intro text missing heading line:\n%s", intro.Text)
	}
	if !strings.Contains(intro.Text, "Intro paragraph.") {
		t.Fatalf("intro text missing body:\n%s", intro.Text)
	}
	if !strings.Contains(intro.Text, "## Goals") || !strings.Contains(intro.Text, "## Implementation") {
		t.Fatalf("intro text missing subheadings:\n%s", intro.Text)
	}
	if strings.Contains(intro.Text, "# Appendix") {
		t.Fatalf("intro text should not include next top-level section:\n%s", intro.Text)
	}

	// Goals chunk should stop at next ##.
	goals := chunks[1]
	if !strings.Contains(goals.Text, "## Goals") {
		t.Fatalf("goals text missing heading line:\n%s", goals.Text)
	}
	if strings.Contains(goals.Text, "## Implementation") {
		t.Fatalf("goals text should not include sibling section:\n%s", goals.Text)
	}
}

