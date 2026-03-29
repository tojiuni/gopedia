package api

import (
	"testing"
)

func TestNormalizeSearchHits(t *testing.T) {
	raw := []map[string]any{
		{
			"l1_id":               "l1-1",
			"l2_id":               "l2-1",
			"matched_l3_id":       "l3-1",
			"l1_title":            "Doc",
			"section_heading":     "# Intro",
			"matched_content":     "hello world",
			"qdrant_score":        0.42,
			"surrounding_context": "more",
			"project_id":          float64(7),
		},
	}
	hits := normalizeSearchHits(raw)
	if len(hits) != 1 {
		t.Fatalf("len=%d", len(hits))
	}
	h := hits[0]
	if h.L1ID != "l1-1" || h.L2ID != "l2-1" || h.L3ID != "l3-1" {
		t.Fatalf("ids: %+v", h)
	}
	if h.Title != "Doc" || h.SectionHeading != "# Intro" {
		t.Fatalf("titles: %+v", h)
	}
	if h.Score != 0.42 {
		t.Fatalf("score=%v", h.Score)
	}
	if h.ProjectID == nil || *h.ProjectID != 7 {
		t.Fatalf("project_id=%v", h.ProjectID)
	}
	if h.Snippet == "" {
		t.Fatal("empty snippet")
	}
}
