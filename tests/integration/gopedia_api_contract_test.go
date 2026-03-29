//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Live API contract checks. Run with:
//
//	GOPEDIA_API_URL=http://127.0.0.1:18787 go test -tags=integration ./tests/integration/... -run GopediaAPI -count=1
func TestGopediaAPIHealthDeps(t *testing.T) {
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("GOPEDIA_API_URL")), "/")
	if base == "" {
		t.Skip("set GOPEDIA_API_URL")
	}
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(base + "/api/health/deps")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Status string                            `json:"status"`
		Deps   map[string]map[string]interface{} `json:"deps"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json: %v body=%s", err, body)
	}
	if out.Status == "" || len(out.Deps) < 4 {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestGopediaAPISearchJSON(t *testing.T) {
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("GOPEDIA_API_URL")), "/")
	if base == "" {
		t.Skip("set GOPEDIA_API_URL")
	}
	c := &http.Client{Timeout: 6 * time.Minute}
	resp, err := c.Get(base + "/api/search?q=test&format=json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	// Either results or failure (empty DB / no OPENAI is ok for contract shape)
	if _, ok := out["results"]; !ok {
		if _, f := out["failure"]; !f {
			t.Fatalf("expected results or failure: %s", body)
		}
	}
}

func TestGopediaAPISearchJSONDetailSummary(t *testing.T) {
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("GOPEDIA_API_URL")), "/")
	if base == "" {
		t.Skip("set GOPEDIA_API_URL")
	}
	c := &http.Client{Timeout: 6 * time.Minute}
	resp, err := c.Get(base + "/api/search?q=test&format=json&detail=summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := out["results"]; !ok {
		if _, f := out["failure"]; !f {
			t.Fatalf("expected results or failure: %s", body)
		}
		return
	}
	results, _ := out["results"].([]interface{})
	if len(results) == 0 {
		return
	}
	r0, _ := results[0].(map[string]interface{})
	if r0 == nil {
		t.Fatalf("result row: %v", results[0])
	}
	if _, bad := r0["surrounding_context"]; bad {
		t.Fatalf("detail=summary must omit surrounding_context: %#v", r0)
	}
	if _, ok := r0["source_path"]; !ok {
		t.Fatalf("detail=summary should include source_path: %#v", r0)
	}
}

func TestGopediaAPISearchJSONInvalidDetail(t *testing.T) {
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("GOPEDIA_API_URL")), "/")
	if base == "" {
		t.Skip("set GOPEDIA_API_URL")
	}
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(base + "/api/search?q=test&format=json&detail=not-a-preset")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
	}
}

func TestGopediaAPISearchJSONInvalidFields(t *testing.T) {
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("GOPEDIA_API_URL")), "/")
	if base == "" {
		t.Skip("set GOPEDIA_API_URL")
	}
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(base + "/api/search?q=test&format=json&fields=not_a_real_key")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
	}
}
