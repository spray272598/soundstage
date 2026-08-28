package moderation

import (
	"context"
	"testing"
)

func TestKeywordModerator(t *testing.T) {
	m := NewKeywordModerator([]string{"广告", "spam"})

	allowed, err := m.Moderate(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed.Allowed {
		t.Fatalf("expected allowed, got rejected: %s", allowed.Reason)
	}

	blocked, err := m.Moderate(context.Background(), "buy my 广告 now")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked.Allowed {
		t.Fatalf("expected blocked for keyword")
	}
	if blocked.Masked == "" {
		t.Fatalf("expected a masked reason")
	}
}
