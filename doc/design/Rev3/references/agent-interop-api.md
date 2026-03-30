# Agent Interop API Contract (Rev3 Reference)

This reference summarizes the current agent-oriented API contract reflected in `doc/guide/agent-interop.md` and implemented in `internal/api`.

## 1. Recommended Agent Call Sequence
1. `GET /api/health`
2. `GET /api/health/deps`
3. `POST /api/ingest/jobs`
4. `GET /api/jobs/{id}` until terminal status
5. `GET /api/search?q=...&format=json` using staged detail controls

## 2. Search JSON Shaping Parameters

### 2.1 `detail`
- `summary`: minimal fields for low-token candidate selection
- `standard`: balanced metadata for grounding
- `full` or omitted: full result object (backward-compatible default)

### 2.2 `fields`
- Comma-separated sparse field selection (e.g. `fields=title,snippet,l3_id`).
- Overrides `detail` when both are provided.

## 3. Preset Field Matrix

| Field | summary | standard | full |
|------|:-------:|:--------:|:----:|
| `doc_id` | O | O | O |
| `project_id` | - | O | O |
| `l1_id` | - | O | O |
| `l2_id` | - | O | O |
| `l3_id` | O | O | O |
| `score` | O | O | O |
| `title` | O | O | O |
| `section_heading` | - | O | O |
| `snippet` | O | O | O |
| `source_path` | O | O | O |
| `breadcrumb` | - | O | O |
| `surrounding_context` | - | - | O (when present) |

## 4. Snippet Semantics
- `snippet` is a short relevance excerpt.
- Source precedence:
  1. `matched_content` when present
  2. fallback to `surrounding_context`
- Truncation is rune-safe and prefers sentence/word boundaries before appending ellipsis.

## 5. Validation & Error Behavior
- Unknown `detail` or invalid/unknown `fields` key: `HTTP 400`.
- Subprocess failures for search/ingest: `HTTP 200` with structured `failure` object.
- Legacy `error` string is still returned for compatibility.
