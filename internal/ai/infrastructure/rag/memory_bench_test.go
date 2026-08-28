package rag

import (
	"context"
	"fmt"
	"testing"

	"github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
)

// benchVec builds a deterministic pseudo-random vector of fixed dimension so
// benchmark runs are reproducible.
func benchVec(seed int) []float32 {
	const d = 32
	v := make([]float32, d)
	x := uint32(seed) + 1
	for i := 0; i < d; i++ {
		x = x*1664525 + 1013904223
		v[i] = float32(int32(x)) / 1e9
	}
	return v
}

func benchIndex(n int) *MemIndex {
	idx := NewMemIndex()
	pts := make([]Point, 0, n)
	for i := 0; i < n; i++ {
		pts = append(pts, Point{ID: fmt.Sprintf("p-%d", i), Vector: benchVec(i)})
	}
	_ = idx.Upsert(context.Background(), pts)
	return idx
}

// BenchmarkMemIndexSearch isolates the cost of the brute-force cosine scan,
// which is the dominant cost of RAG retrieval as the rulebook grows.
func BenchmarkMemIndexSearch(b *testing.B) {
	const n = 2000
	idx := benchIndex(n)
	q := benchVec(777)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Search(context.Background(), q, 5)
	}
}

// BenchmarkKnowledgeQuery measures the full RAG path (embed + search + filter),
// using the offline MockEmbedder so the number reflects single-node retrieval
// cost without a network round-trip.
func BenchmarkKnowledgeQuery(b *testing.B) {
	emb := llm.NewEmbedderFromConfig("", "", "")
	svc := NewService(emb)
	ctx := context.Background()
	for i := 0; i < 200; i++ {
		_ = svc.Index(ctx, fmt.Sprintf("doc-%d", i), "rule",
			fmt.Sprintf("audio room rule number %d about conduct, gifts and mute", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Query(ctx, "禁言规则是什么", 3)
	}
}
