package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// APIError is a machine-readable error payload for agents and structured clients.
type APIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable"`
	RequestID string         `json:"request_id,omitempty"`
}

// SearchHit is a normalized retrieval result for JSON search responses.
type SearchHit struct {
	DocID              string  `json:"doc_id"`
	ProjectID          *int64  `json:"project_id,omitempty"`
	DocName            string  `json:"doc_name,omitempty"`
	L1ID               string  `json:"l1_id"`
	L2ID               string  `json:"l2_id"`
	L3ID               string  `json:"l3_id"`
	Score              float64 `json:"score"`
	Title              string  `json:"title"`
	SectionHeading     string  `json:"section_heading"`
	Snippet            string  `json:"snippet"`
	SourcePath         string  `json:"source_path"`
	Breadcrumb         string  `json:"breadcrumb,omitempty"`
	SurroundingContext string  `json:"surrounding_context,omitempty"`
}

// AnswerResponse is the JSON body for GET /api/answer.
type AnswerResponse struct {
	Answer    string          `json:"answer,omitempty"`
	Sources   []string        `json:"sources,omitempty"`
	Found     bool            `json:"found"`
	Trace     []string        `json:"trace,omitempty"`
	Stderr    string          `json:"stderr,omitempty"`
	Error     string          `json:"error,omitempty"`
	Failure   *APIError       `json:"failure,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

// RestoreResponse is the JSON body for GET /api/restore.
type RestoreResponse struct {
	Markdown  string          `json:"markdown,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Stderr    string          `json:"stderr,omitempty"`
	Error     string          `json:"error,omitempty"`
	Failure   *APIError       `json:"failure,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

// IndexResetRequest is the JSON body for POST /api/index/reset.
type IndexResetRequest struct {
	ProjectID *int64 `json:"project_id,omitempty"`
	DryRun    bool   `json:"dry_run"`
}

// IndexResetResponse is the JSON body returned by POST /api/index/reset.
type IndexResetResponse struct {
	Result    json.RawMessage `json:"result,omitempty"`
	Stderr    string          `json:"stderr,omitempty"`
	Error     string          `json:"error,omitempty"`
	Failure   *APIError       `json:"failure,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

// IngestJobRequest is the body for POST /api/ingest/jobs.
type IngestJobRequest struct {
	Path      string `json:"path"`
	ProjectID *int64 `json:"project_id,omitempty"`
}

// IngestJobCreateResponse is returned when a job is accepted.
type IngestJobCreateResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

// IngestJobStatusResponse is returned by GET /api/jobs/{id}.
type IngestJobStatusResponse struct {
	JobID     string         `json:"job_id"`
	Status    string         `json:"status"`
	Progress  map[string]int `json:"progress,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	StartedAt string         `json:"started_at,omitempty"`
	EndedAt   string         `json:"ended_at,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
	Failure   *APIError      `json:"failure,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

// HealthDepsResponse is returned by GET /api/health/deps.
type HealthDepsResponse struct {
	Status    string               `json:"status"`
	Deps      map[string]DepStatus `json:"deps"`
	RequestID string               `json:"request_id,omitempty"`
}

// DepStatus is one dependency check result for /api/health/deps.
type DepStatus struct {
	Status     string `json:"status"`
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	CheckLevel string `json:"check_level,omitempty"`
	Error      string `json:"error,omitempty"`
}

// normalizeSearchHits maps Python retrieve_and_enrich JSON objects to SearchHit.
func normalizeSearchHits(raw []map[string]any) []SearchHit {
	out := make([]SearchHit, 0, len(raw))
	for _, m := range raw {
		h := SearchHit{
			DocID:              stringField(m, "doc_id"),
			DocName:            stringField(m, "doc_name"),
			L1ID:               stringField(m, "l1_id"),
			L2ID:               stringField(m, "l2_id"),
			L3ID:               coalesce(stringField(m, "l3_id"), stringField(m, "matched_l3_id")),
			Title:              stringField(m, "l1_title"),
			SectionHeading:     stringField(m, "section_heading"),
			Breadcrumb:         stringField(m, "breadcrumb"),
			SurroundingContext: stringField(m, "surrounding_context"),
			SourcePath:         stringField(m, "source_path"),
		}
		if h.L3ID == "" {
			continue
		}
		h.Score = floatField(m, "qdrant_score")
		snippet := stringField(m, "matched_content")
		if snippet == "" {
			snippet = trimSnippet(h.SurroundingContext, 500)
		} else {
			snippet = trimSnippet(snippet, 500)
		}
		h.Snippet = snippet
		if v, ok := m["project_id"]; ok && v != nil {
			if pid, err := toInt64(v); err == nil {
				h.ProjectID = &pid
			}
		}
		out = append(out, h)
	}
	return out
}

// trimSnippet returns a short excerpt of at most max runes, preferring sentence or word boundaries
// so UTF-8 text is not cut mid-rune.
func trimSnippet(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	cut := runes[:max]
	minIdx := int(float64(len(cut)) * 0.4)

	end := len(cut)
	found := false
	for i := len(cut) - 1; i >= minIdx && !found; i-- {
		switch cut[i] {
		case '.', '?', '!', '。', '\n':
			end = i + 1
			found = true
		}
	}
	if !found {
		for i := len(cut) - 1; i >= minIdx; i-- {
			if unicode.IsSpace(cut[i]) {
				end = i
				break
			}
		}
	}
	out := strings.TrimSpace(string(cut[:end]))
	if out == "" {
		out = strings.TrimSpace(string(cut))
	}
	return out + "…"
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func floatField(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		s := fmt.Sprint(t)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return f
	}
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func newAPIError(code, message string, retryable bool, requestID string, details map[string]any) *APIError {
	return &APIError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
		RequestID: requestID,
		Details:   details,
	}
}

func toInt64(v any) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case float64:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	case string:
		return strconv.ParseInt(strings.TrimSpace(t), 10, 64)
	default:
		return strconv.ParseInt(fmt.Sprint(t), 10, 64)
	}
}
