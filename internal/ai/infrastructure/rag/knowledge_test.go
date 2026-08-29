package rag

import (
	"context"
	"testing"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	aillm "github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
)

func TestMemIndexSearch(t *testing.T) {
	idx := NewMemIndex()
	_ = idx.Upsert(context.Background(), []aidomain.VectorPoint{
		{ID: "a", Vector: []float32{1, 0, 0}},
		{ID: "b", Vector: []float32{0, 1, 0}},
		{ID: "c", Vector: []float32{0, 0, 1}},
	})
	hits, err := idx.Search(context.Background(), []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].ID != "a" {
		t.Fatalf("expected top hit a, got %s", hits[0].ID)
	}
	if hits[0].Score < hits[1].Score {
		t.Fatal("top hit should have the highest score")
	}
}

func TestKnowledgeBaseQuery(t *testing.T) {
	kb := NewService(aillm.NewMockEmbedder(32))
	ctx := context.Background()
	if err := kb.Index(ctx, "r1", "规则", "直播间禁止发布违法违规内容"); err != nil {
		t.Fatal(err)
	}
	if err := kb.Index(ctx, "r2", "礼物", "观众可以送礼物支持主播"); err != nil {
		t.Fatal(err)
	}
	chunks, err := kb.Query(ctx, "违规内容如何处理", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if chunks[0].ID != "r1" {
		t.Fatalf("expected rule chunk first, got %s", chunks[0].ID)
	}
}

func TestKnowledgeBaseSeedCount(t *testing.T) {
	kb := NewService(aillm.NewMockEmbedder(32))
	if err := SeedDefaultKnowledge(context.Background(), kb); err != nil {
		t.Fatal(err)
	}
	n, _ := kb.Count(context.Background())
	if n != len(DefaultDocuments()) {
		t.Fatalf("expected %d seeded docs, got %d", len(DefaultDocuments()), n)
	}
}
