// Package agent implements the ai domain.Agent port: a ReAct tool-calling
// loop driven by a Gateway and a Registry of Tools. It is fully decoupled from
// any concrete provider or external context.
package agent

import (
	"sync"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// MapRegistry is a thread-safe in-memory tool registry.
type MapRegistry struct {
	mu    sync.RWMutex
	tools map[string]aidomain.Tool
}

// NewMapRegistry builds an empty registry.
func NewMapRegistry() *MapRegistry {
	return &MapRegistry{tools: make(map[string]aidomain.Tool)}
}

// Register adds a tool (replaces any existing one with the same name).
func (r *MapRegistry) Register(t aidomain.Tool) {
	if t == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get returns a tool by name.
func (r *MapRegistry) Get(name string) (aidomain.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *MapRegistry) List() []aidomain.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]aidomain.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Specs returns the OpenAI-style schemas of every tool.
func (r *MapRegistry) Specs() []aidomain.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]aidomain.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, aidomain.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.InputSchema(),
		})
	}
	return out
}

// Compile-time check.
var _ aidomain.Registry = (*MapRegistry)(nil)
