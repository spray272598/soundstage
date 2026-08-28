package domain

import "context"

// Chunk is one retrieved passage from the knowledge base, with its similarity
// score and source title for citation in the AI reply.
type Chunk struct {
	ID    string
	Title string
	Text  string
	Score float32
}

// KnowledgeBase is the RAG retrieval port. Chunks are embedded on Index and
// matched by cosine similarity on Query. Implemented in infrastructure/rag.
type KnowledgeBase interface {
	// Index stores one document chunk (embedding + upsert into the vector index).
	Index(ctx context.Context, id, title, text string) error
	// Query returns the topK most similar chunks to the query string.
	Query(ctx context.Context, query string, topK int) ([]Chunk, error)
	// Count returns how many chunks are currently indexed.
	Count(ctx context.Context) (int, error)
}
