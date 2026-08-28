package moderation

import (
	"context"
	"strings"

	"github.com/spray272598/soundstage/internal/interaction/domain"
)

// KeywordModerator is the default, dependency-free moderator: it blocks any
// message containing a configured keyword. The AI context (Phase 4) can
// replace it with an LLM-backed moderator behind the same domain.Moderator
// port, so the application layer never changes.
type KeywordModerator struct {
	keywords []string
}

// NewKeywordModerator creates a KeywordModerator from a blocklist.
func NewKeywordModerator(keywords []string) *KeywordModerator {
	return &KeywordModerator{keywords: keywords}
}

// Moderate returns a rejection decision when a blocked keyword is present.
func (m *KeywordModerator) Moderate(ctx context.Context, content string) (domain.ModerationDecision, error) {
	for _, kw := range m.keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(content, kw) {
			return domain.ModerationDecision{
				Allowed: false,
				Reason:  "contains blocked keyword: " + kw,
				Masked:  mask(content, kw),
			}, nil
		}
	}
	return domain.ModerationDecision{Allowed: true, Masked: content}, nil
}

func mask(content, kw string) string {
	return strings.ReplaceAll(content, kw, strings.Repeat("*", len([]rune(kw))))
}

// Compile-time check.
var _ domain.Moderator = (*KeywordModerator)(nil)
