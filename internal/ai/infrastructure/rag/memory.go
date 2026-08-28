// Package rag implements the ai domain.KnowledgeBase port: an in-process
// brute-force cosine index backed by an Embedder. Single-node and good enough
// for a room's rulebook/FAQ; a future remote backend (Qdrant/pgvector) can
// replace MemIndex without touching call sites.
package rag

import (
	"context"
	"sort"
	"sync"
)

// Point is one vector record.
type Point struct {
	ID      string
	Vector  []float32
	Payload map[string]any
}

// Hit is a scored search result (descending similarity).
type Hit struct {
	ID      string
	Score   float32
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

// Upsert inserts or replaces points by id.
func (m *MemIndex) Upsert(_ context.Context, points []Point) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range points {
		m.collection[p.ID] = p
	}
	return nil
}

// Search returns the topK most similar points to query.
func (m *MemIndex) Search(_ context.Context, query []float32, topK int) ([]Hit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hits := make([]Hit, 0, len(m.collection))
	for _, p := range m.collection {
		hits = append(hits, Hit{ID: p.ID, Score: cosine(query, p.Vector), Payload: p.Payload})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

// Count returns the number of indexed points.
func (m *MemIndex) Count(_ context.Context) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.collection)
}

// Delete removes a point by id.
func (m *MemIndex) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.collection, id)
	return nil
}

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
