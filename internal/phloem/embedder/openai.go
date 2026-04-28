package embedder

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

const defaultOpenAIBase = "https://api.openai.com/v1"

// OpenAI is the OpenAI embeddings implementation.
// Set OPENAI_BASE_URL to override the API base (e.g. for Ollama compatibility).
type OpenAI struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAI from env OPENAI_API_KEY, OPENAI_EMBEDDING_MODEL (default text-embedding-3-small).
// OPENAI_BASE_URL overrides the API base URL (default: https://api.openai.com/v1).
func NewOpenAI() *OpenAI {
	model := os.Getenv("OPENAI_EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	baseURL := strings.TrimRight(os.Getenv("OPENAI_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBase
	}
	return &OpenAI{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

type openAIEmbedReq struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type openAIEmbedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed implements Embedder.
func (e *OpenAI) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	body := openAIEmbedReq{Model: e.model, Input: text}
	jb, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(jb))
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

// EmbedMany returns embeddings for multiple texts in one API call.
func (e *OpenAI) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
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
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(jb))
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

// VectorSize returns the expected embedding dimension for the configured model.
func (e *OpenAI) VectorSize() int {
	switch {
	case strings.Contains(e.model, "3-large"):
		return 3072
	case strings.Contains(e.model, "nomic-embed"):
		return 768
	case strings.Contains(e.model, "bge-m3"):
		return 1024
	default:
		return 1536
	}
}
