package rag

import (
	"context"
	"fmt"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
)

// Service is the RAG knowledge base: it embeds documents into a VectorStore and
// retrieves the most relevant chunks for a query.
type Service struct {
	embedder llm.Embedder
	index    aidomain.VectorStore
	// minScore filters near-irrelevant hits so the model isn't fed noise.
	minScore float32
}

// NewService builds a RAG service from an embedder (uses in-memory MemIndex).
func NewService(embedder llm.Embedder) *Service {
	return &Service{
		embedder: embedder,
		index:    NewMemIndex(),
		minScore: 0.1,
	}
}

// NewServiceWithVectorStore builds a RAG service with a custom VectorStore (e.g., pgvector).
func NewServiceWithVectorStore(embedder llm.Embedder, index aidomain.VectorStore) *Service {
	return &Service{
		embedder: embedder,
		index:    index,
		minScore: 0.1,
	}
}

// Index embeds a single document and upserts it into the vector index.
func (s *Service) Index(ctx context.Context, id, title, text string) error {
	vecs, err := s.embedder.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("rag index embed: %w", err)
	}
	if len(vecs) == 0 {
		return fmt.Errorf("rag index: empty embedding")
	}
	return s.index.Upsert(ctx, []aidomain.VectorPoint{{
		ID:      id,
		Vector:  vecs[0],
		Payload: map[string]any{"title": title, "text": text},
	}})
}

// Query embeds the query and returns the topK most similar chunks.
func (s *Service) Query(ctx context.Context, query string, topK int) ([]aidomain.Chunk, error) {
	vecs, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("rag query embed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	hits, err := s.index.Search(ctx, vecs[0], topK)
	if err != nil {
		return nil, err
	}
	chunks := make([]aidomain.Chunk, 0, len(hits))
	for _, h := range hits {
		if h.Score < s.minScore {
			continue
		}
		chunks = append(chunks, aidomain.Chunk{
			ID:    h.ID,
			Title: titleOf(h.Payload),
			Text:  textOf(h.Payload),
			Score: h.Score,
		})
	}
	return chunks, nil
}

// Count returns the number of indexed chunks.
func (s *Service) Count(ctx context.Context) (int, error) {
	return s.index.Count(ctx)
}

func titleOf(p map[string]any) string {
	if v, ok := p["title"].(string); ok {
		return v
	}
	return ""
}

func textOf(p map[string]any) string {
	if v, ok := p["text"].(string); ok {
		return v
	}
	return ""
}

// Compile-time check.
var _ aidomain.KnowledgeBase = (*Service)(nil)
