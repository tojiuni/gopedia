package phloem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Embedder produces vectors for text (OpenAI embeddings).
type Embedder struct {
	apiKey string
	model  string
	client *http.Client
}

// NewEmbedder from env OPENAI_API_KEY, OPENAI_EMBEDDING_MODEL (default text-embedding-3-small).
func NewEmbedder() *Embedder {
	model := os.Getenv("OPENAI_EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &Embedder{
		apiKey: os.Getenv("OPENAI_API_KEY"),
		model:  model,
		client: &http.Client{},
	}
}

// openAIEmbedReq matches the API request body.
type openAIEmbedReq struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string or []string
}

// openAIEmbedResp matches the API response.
type openAIEmbedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns the embedding vector for the given text. Returns nil if API key is unset or on error.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	body := openAIEmbedReq{Model: e.model, Input: text}
	jb, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(jb))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embeddings: %s: %s", resp.Status, string(b))
	}
	var out openAIEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("openai: no embedding in response")
	}
	return out.Data[0].Embedding, nil
}

// EmbedMany returns embeddings for multiple texts in one API call when possible.
func (e *Embedder) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) == 1 {
		vec, err := e.Embed(ctx, texts[0])
		if err != nil {
			return nil, err
		}
		return [][]float32{vec}, nil
	}
	body := openAIEmbedReq{Model: e.model, Input: texts}
	jb, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(jb))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embeddings: %s: %s", resp.Status, string(b))
	}
	var out openAIEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("openai: got %d embeddings, want %d", len(out.Data), len(texts))
	}
	result := make([][]float32, len(texts))
	for i := range out.Data {
		result[i] = out.Data[i].Embedding
	}
	return result, nil
}

// VectorSize returns 1536 for text-embedding-3-small/ada-002, else 1536 as default.
func (e *Embedder) VectorSize() int {
	if strings.Contains(e.model, "3-large") {
		return 3072
	}
	return 1536
}
