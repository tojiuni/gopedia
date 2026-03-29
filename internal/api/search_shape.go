package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// allowedSearchResultJSONKeys are JSON keys agents may request via ?fields= (and used by detail presets).
var allowedSearchResultJSONKeys = map[string]struct{}{
	"doc_id":              {},
	"project_id":          {},
	"l1_id":               {},
	"l2_id":               {},
	"l3_id":               {},
	"score":               {},
	"title":               {},
	"section_heading":     {},
	"snippet":             {},
	"source_path":         {},
	"breadcrumb":          {},
	"surrounding_context": {},
}

// parseSearchResultFields returns nil when the client wants the full default JSON shape (all SearchHit fields).
// Non-nil is an ordered list of JSON object keys per hit. If ?fields= is non-empty, it overrides ?detail=.
func parseSearchResultFields(detail, fieldsCSV string) ([]string, error) {
	if s := strings.TrimSpace(fieldsCSV); s != "" {
		return parseSearchFieldsCSV(s)
	}
	switch strings.ToLower(strings.TrimSpace(detail)) {
	case "", "full":
		return nil, nil
	case "summary":
		return []string{"doc_id", "l3_id", "score", "title", "snippet", "source_path"}, nil
	case "standard":
		return []string{
			"doc_id", "project_id", "l1_id", "l2_id", "l3_id", "score", "title",
			"section_heading", "snippet", "source_path", "breadcrumb",
		}, nil
	default:
		return nil, fmt.Errorf("invalid detail (use summary, standard, full, or omit)")
	}
}

func parseSearchFieldsCSV(fieldsCSV string) ([]string, error) {
	parts := strings.Split(fieldsCSV, ",")
	seen := make(map[string]struct{}, len(parts))
	var out []string
	for _, p := range parts {
		k := strings.ToLower(strings.TrimSpace(p))
		if k == "" {
			continue
		}
		if _, ok := allowedSearchResultJSONKeys[k]; !ok {
			return nil, fmt.Errorf("unknown fields key %q", k)
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("fields must list at least one valid key")
	}
	return out, nil
}

func searchHitToMap(h SearchHit, keys []string) map[string]any {
	m := make(map[string]any, len(keys))
	for _, k := range keys {
		switch k {
		case "doc_id":
			m["doc_id"] = h.DocID
		case "project_id":
			if h.ProjectID != nil {
				m["project_id"] = *h.ProjectID
			}
		case "l1_id":
			m["l1_id"] = h.L1ID
		case "l2_id":
			m["l2_id"] = h.L2ID
		case "l3_id":
			m["l3_id"] = h.L3ID
		case "score":
			m["score"] = h.Score
		case "title":
			m["title"] = h.Title
		case "section_heading":
			m["section_heading"] = h.SectionHeading
		case "snippet":
			m["snippet"] = h.Snippet
		case "source_path":
			m["source_path"] = h.SourcePath
		case "breadcrumb":
			m["breadcrumb"] = h.Breadcrumb
		case "surrounding_context":
			m["surrounding_context"] = h.SurroundingContext
		}
	}
	return m
}

// marshalSearchResults encodes hits for GET /api/search?format=json.
// keys == nil means marshal []SearchHit (backward-compatible full shape).
func marshalSearchResults(hits []SearchHit, keys []string) (json.RawMessage, error) {
	if keys == nil {
		return json.Marshal(hits)
	}
	out := make([]map[string]any, len(hits))
	for i := range hits {
		out[i] = searchHitToMap(hits[i], keys)
	}
	return json.Marshal(out)
}
