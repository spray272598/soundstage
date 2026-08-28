package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Embedder turns text into dense vectors for the RAG index. OpenAI-compatible
// endpoints (OpenAI, SiliconFlow, etc.) share the same /embeddings shape.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dims() int
}

// OpenAIEmbedder calls an OpenAI-compatible /embeddings endpoint.
type OpenAIEmbedder struct {
	apiKey  string
	apiBase string
	model   string
	dims    int
	client  *http.Client
}

// NewOpenAIEmbedder builds an embedder. apiBase defaults to the OpenAI v1 URL;
// model defaults to text-embedding-3-small.
func NewOpenAIEmbedder(apiKey, apiBase, model string) *OpenAIEmbedder {
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	apiBase = strings.TrimRight(apiBase, "/")
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		dims:    1536,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Dims returns the embedding dimensionality.
func (e *OpenAIEmbedder) Dims() int { return e.dims }

// Embed requests vectors for the given texts in one batch.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("embedding: no api key configured")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	body, _ := json.Marshal(map[string]any{
		"model":           e.model,
		"input":           texts,
		"encoding_format": "float",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiBase+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding http %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embedding: got %d vectors for %d inputs", len(out.Data), len(texts))
	}
	vecs := make([][]float32, 0, len(out.Data))
	for _, d := range out.Data {
		vecs = append(vecs, d.Embedding)
	}
	return vecs, nil
}

// MockEmbedder is a deterministic, dependency-free embedder for offline demos
// and tests. It hashes character n-grams into a fixed-dim vector and L2-
// normalizes it, so semantically similar phrases land closer in cosine space.
type MockEmbedder struct {
	dims int
}

// NewMockEmbedder builds a MockEmbedder with the given dimensionality.
func NewMockEmbedder(dims int) *MockEmbedder {
	if dims <= 0 {
		dims = 32
	}
	return &MockEmbedder{dims: dims}
}

// Dims returns the embedding dimensionality.
func (m *MockEmbedder) Dims() int { return m.dims }

// Embed produces hashed vectors for each text.
func (m *MockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, t := range texts {
		out = append(out, m.embedOne(t))
	}
	return out, nil
}

func (m *MockEmbedder) embedOne(text string) []float32 {
	v := make([]float32, m.dims)
	lower := strings.ToLower(strings.TrimSpace(text))
	runes := []rune(lower)
	// Unigrams + bigrams so shared words/phrases increase similarity.
	for i := 0; i < len(runes); i++ {
		h := hashRunes(runes[i])
		v[h%m.dims] += 1.0
		if i+1 < len(runes) {
			h2 := hashRunes(runes[i])*31 + hashRunes(runes[i+1])
			v[h2%m.dims] += 0.7
		}
	}
	normalize(v)
	return v
}

func hashRunes(r rune) int {
	return int(r)
}

func normalize(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	n := float32(1.0 / math.Sqrt(sum))
	for i := range v {
		v[i] *= n
	}
}

// NewEmbedderFromConfig selects the real embedder when a key is present, else
// the deterministic mock so the RAG pipeline still works offline.
func NewEmbedderFromConfig(apiKey, apiBase, model string) Embedder {
	if apiKey == "" {
		return NewMockEmbedder(32)
	}
	return NewOpenAIEmbedder(apiKey, apiBase, model)
}

// Compile-time checks.
var (
	_ Embedder = (*OpenAIEmbedder)(nil)
	_ Embedder = (*MockEmbedder)(nil)
)
