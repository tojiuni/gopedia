package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Local calls a local embedding HTTP service (e.g. python/embedding_service).
// Set LOCAL_EMBEDDING_ADDR to the base URL (default: http://localhost:18789).
// Documents are embedded with prefix "passage"; queries with prefix "query".
type Local struct {
	addr   string
	client *http.Client
}

// NewLocal from env LOCAL_EMBEDDING_ADDR (default http://localhost:18789).
func NewLocal() *Local {
	addr := os.Getenv("LOCAL_EMBEDDING_ADDR")
	if addr == "" {
		addr = "http://localhost:18789"
	}
	return &Local{addr: addr, client: &http.Client{}}
}

type localEmbedReq struct {
	Texts  []string `json:"texts"`
	Prefix string   `json:"prefix"`
}

type localEmbedResp struct {
	Embeddings [][]float32 `json:"embeddings"`
	VectorSize int         `json:"vector_size"`
}

// Embed implements Embedder. Uses "passage" prefix for document ingestion.
func (e *Local) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedMany(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("local embedding: empty response")
	}
	return vecs[0], nil
}

// EmbedMany returns embeddings for multiple texts in one request.
func (e *Local) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body := localEmbedReq{Texts: texts, Prefix: "passage"}
	jb, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", e.addr+"/embed", bytes.NewReader(jb))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("local embedding: %s: %s", resp.Status, string(b))
	}
	var out localEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embeddings, nil
}

// VectorSize returns 1024 for multilingual-e5-large.
func (e *Local) VectorSize() int {
	return 1024
}
