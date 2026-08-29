package domain

import "context"

// VectorStore is the port for vector similarity search backends.
// Implementations can be in-memory (MemIndex), pgvector, Qdrant, etc.
type VectorStore interface {
	// Upsert inserts or updates vectors with their payloads.
	Upsert(ctx context.Context, points []VectorPoint) error

	// Search returns the topK most similar vectors to the query vector.
	Search(ctx context.Context, query []float32, topK int) ([]VectorHit, error)

	// Delete removes vectors by ID.
	Delete(ctx context.Context, ids []string) error

	// Count returns the number of stored vectors.
	Count(ctx context.Context) (int, error)

	// Close releases any resources (connections, etc.).
	Close() error
}

// VectorPoint is a single vector record with metadata.
type VectorPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]any
}

// VectorHit is a scored search result.
type VectorHit struct {
	ID      string
	Score   float32
	Payload map[string]any
}

// VectorStoreConfig holds configuration for vector store implementations.
type VectorStoreConfig struct {
	// Type: "memory" | "pgvector" | "qdrant"
	Type string `mapstructure:"type"`

	// PGVectorConfig holds pgvector-specific settings.
	PGVector PGVectorConfig `mapstructure:"pgvector"`

	// QdrantConfig holds Qdrant-specific settings.
	Qdrant QdrantConfig `mapstructure:"qdrant"`
}

// PGVectorConfig holds pgvector connection settings.
type PGVectorConfig struct {
	DSN          string `mapstructure:"dsn"`
	TableName    string `mapstructure:"table_name"`    // default: "ai_knowledge_base"
	VectorDims   int    `mapstructure:"vector_dims"`   // default: 1536
	PoolSize     int    `mapstructure:"pool_size"`     // default: 10
	HNSWEfSearch int    `mapstructure:"hnsw_ef_search"` // default: 64
}

// QdrantConfig holds Qdrant connection settings.
type QdrantConfig struct {
	URL          string `mapstructure:"url"`
	APIKey       string `mapstructure:"api_key"`
	Collection   string `mapstructure:"collection"`    // default: "ai_knowledge_base"
	VectorDims   int    `mapstructure:"vector_dims"`   // default: 1536
	Timeout      int    `mapstructure:"timeout"`       // default: 30 (seconds)
}