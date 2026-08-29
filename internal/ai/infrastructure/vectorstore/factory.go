package vectorstore

import (
	"context"
	"fmt"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/rag"
)

// VectorStoreFactory creates VectorStore instances based on configuration.
type VectorStoreFactory struct{}

// NewVectorStore creates a VectorStore from config.
// Falls back to in-memory MemIndex if type is "memory" or empty.
func (f *VectorStoreFactory) NewVectorStore(ctx context.Context, cfg aidomain.VectorStoreConfig, embedder llm.Embedder) (aidomain.VectorStore, error) {
	switch cfg.Type {
	case "pgvector":
		return NewPGVectorStore(ctx, cfg.PGVector)
	case "qdrant":
		return nil, fmt.Errorf("qdrant not yet implemented")
	case "", "memory":
		return rag.NewMemIndex(), nil
	default:
		return nil, fmt.Errorf("unknown vector store type: %s", cfg.Type)
	}
}

// NewKnowledgeBaseService creates a RAG KnowledgeBase service with the configured vector store.
func NewKnowledgeBaseService(ctx context.Context, cfg aidomain.VectorStoreConfig, embedder llm.Embedder) (*rag.Service, error) {
	factory := &VectorStoreFactory{}
	vectorStore, err := factory.NewVectorStore(ctx, cfg, embedder)
	if err != nil {
		return nil, err
	}

	// Wrap the vector store in a KnowledgeBase-compatible adapter
	kb := rag.NewServiceWithVectorStore(embedder, vectorStore)
	return kb, nil
}