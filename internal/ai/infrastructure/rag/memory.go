// Package rag implements the ai domain.KnowledgeBase port: an in-process
// brute-force cosine index backed by an Embedder. Single-node and good enough
// for a room's rulebook/FAQ; a future remote backend (Qdrant/pgvector) can
// replace MemIndex without touching call sites.
package rag

import (
	"context"
	"sort"
	"sync"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// Point is one vector record (internal representation).
type Point struct {
	ID      string
	Vector  []float32
	Payload map[string]any
}

// MemIndex is a concurrency-safe brute-force cosine index.
type MemIndex struct {
	mu         sync.RWMutex
	collection map[string]Point
}

// NewMemIndex returns an empty index.
func NewMemIndex() *MemIndex {
	return &MemIndex{collection: make(map[string]Point)}
}

// --- VectorStore interface implementation ---

// Upsert implements aidomain.VectorStore.Upsert
func (m *MemIndex) Upsert(ctx context.Context, points []aidomain.VectorPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range points {
		m.collection[p.ID] = Point{ID: p.ID, Vector: p.Vector, Payload: p.Payload}
	}
	return nil
}

// Search implements aidomain.VectorStore.Search
func (m *MemIndex) Search(ctx context.Context, query []float32, topK int) ([]aidomain.VectorHit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hits := make([]aidomain.VectorHit, 0, len(m.collection))
	for _, p := range m.collection {
		hits = append(hits, aidomain.VectorHit{ID: p.ID, Score: cosine(query, p.Vector), Payload: p.Payload})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

// Delete implements aidomain.VectorStore.Delete
func (m *MemIndex) Delete(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.collection, id)
	}
	return nil
}

// Count implements aidomain.VectorStore.Count
func (m *MemIndex) Count(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.collection), nil
}

// Close implements aidomain.VectorStore.Close (no-op for in-memory)
func (m *MemIndex) Close() error {
	return nil
}

// Compile-time check.
var _ aidomain.VectorStore = (*MemIndex)(nil)

func cosine(a, b []float32) float32 {
	n := len(a)
	if n > len(b) {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (sqrt(na) * sqrt(nb)))
}

func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 16; i++ {
		z = (z + x/z) / 2
	}
	return z
}
