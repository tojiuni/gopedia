package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSearchResultFields(t *testing.T) {
	t.Parallel()
	keys, err := parseSearchResultFields("", "")
	if err != nil || keys != nil {
		t.Fatalf("empty: keys=%v err=%v", keys, err)
	}
	keys, err = parseSearchResultFields("full", "")
	if err != nil || keys != nil {
		t.Fatalf("full: keys=%v err=%v", keys, err)
	}
	keys, err = parseSearchResultFields("summary", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 6 || keys[5] != "source_path" {
		t.Fatalf("summary keys: %v", keys)
	}
	keys, err = parseSearchResultFields("standard", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 11 {
		t.Fatalf("standard len: %v", keys)
	}
	_, err = parseSearchResultFields("nope", "")
	if err == nil {
		t.Fatal("expected error")
	}
	keys, err = parseSearchResultFields("summary", "l3_id, title")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "l3_id" || keys[1] != "title" {
		t.Fatalf("fields overrides detail: %v", keys)
	}
	_, err = parseSearchResultFields("", "notafield")
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	_, err = parseSearchResultFields("", "  ,  ")
	if err == nil {
		t.Fatal("expected empty fields error")
	}
}

func TestMarshalSearchResultsSparse(t *testing.T) {
	t.Parallel()
	hits := []SearchHit{{
		DocID: "d1", L1ID: "a", L2ID: "b", L3ID: "c", Score: 0.5,
		Title: "T", SectionHeading: "S", Snippet: "sn", SourcePath: "/p",
		Breadcrumb: "br", SurroundingContext: "ctx",
	}}
	pid := int64(3)
	hits[0].ProjectID = &pid

	raw, err := marshalSearchResults(hits, []string{"doc_id", "snippet", "source_path"})
	if err != nil {
		t.Fatal(err)
	}
	var dec []map[string]any
	if err := json.Unmarshal(raw, &dec); err != nil {
		t.Fatal(err)
	}
	if len(dec) != 1 {
		t.Fatalf("len %d", len(dec))
	}
	if dec[0]["l1_id"] != nil {
		t.Fatalf("should omit l1_id: %#v", dec[0])
	}
	if dec[0]["doc_id"] != "d1" || dec[0]["snippet"] != "sn" || dec[0]["source_path"] != "/p" {
		t.Fatalf("unexpected: %#v", dec[0])
	}
}

func TestTrimSnippet(t *testing.T) {
	t.Parallel()
	if got := trimSnippet("hi", 10); got != "hi" {
		t.Fatalf("short: %q", got)
	}
	if got := trimSnippet("", 10); got != "" {
		t.Fatalf("empty: %q", got)
	}
	long := strings.Repeat("word ", 200) + "end."
	out := trimSnippet(long, 80)
	if !strings.HasSuffix(out, "…") {
		t.Fatalf("expected ellipsis: %q", out)
	}
	if strings.Contains(out, "\uFFFD") {
		t.Fatalf("replacement char in output: %q", out)
	}
	// Rune-safe: 600 Korean syllables, cap 120 runes — must not panic or corrupt UTF-8.
	ko := strings.Repeat("가", 600)
	out = trimSnippet(ko, 120)
	runes := []rune(out)
	if len(runes) > 121 { // 120 + ellipsis
		t.Fatalf("too long: rune len %d", len(runes))
	}
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("invalid string for JSON: %v", err)
	}
}
