package agentframework

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ParamSchema is a minimal, JSON-schema-ish description of one named
// parameter a Tool accepts. This package does not depend on a JSON-schema
// library — Type is a free-form string (e.g. "string", "integer",
// "number", "boolean", "array") intended for a model-facing tool
// manifest, not runtime validation. A Tool's Invoke function is
// responsible for validating its own args at call time.
type ParamSchema struct {
	// Name is the argument key expected in a ToolCall's Args map.
	Name string

	// Type is a free-form JSON-schema-ish type hint (e.g. "string").
	Type string

	// Description explains the parameter's purpose to a model deciding
	// how to call this tool.
	Description string

	// Required indicates whether Invoke expects this argument to be
	// present.
	Required bool
}

// ToolResult is the outcome of a single Tool.Invoke call, recorded on the
// Scratchpad as an Observation.
type ToolResult struct {
	// Content is the tool's output, rendered as a string suitable for
	// inclusion in a subsequent model prompt.
	Content string

	// Data optionally carries the tool's structured output for callers
	// that want more than the rendered Content (e.g. an orchestration
	// pipeline inspecting a specific field). May be nil.
	Data any
}

// Tool is a single named capability an Agent may invoke during a step,
// typically wrapping one knowledgeapi.KnowledgeAPI method (see
// tools_knowledgeapi.go) so that no agent built on this framework
// reimplements retrieval logic itself.
type Tool struct {
	// Name uniquely identifies the tool within a ToolSet. Must be
	// non-empty.
	Name string

	// Description explains what the tool does, for inclusion in a
	// model-facing tool manifest.
	Description string

	// Parameters documents the arguments Invoke accepts.
	Parameters []ParamSchema

	// Invoke performs the tool's action. Implementations must honor ctx
	// cancellation/deadlines and should return an error wrapping a
	// specific sentinel where one is available, rather than a bare
	// fmt.Errorf, so ToolSet.Invoke's ErrToolInvocation wrapping still
	// preserves the underlying cause for errors.Is.
	Invoke func(ctx context.Context, args map[string]any) (ToolResult, error)
}

// validate reports whether t is well-formed enough to register.
func (t Tool) validate() error {
	if t.Name == "" || t.Invoke == nil {
		return ErrNilTool
	}
	return nil
}

// ToolSet is a registry of Tool values an Agent's ToolCalls are dispatched
// against. Safe for concurrent use.
type ToolSet struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolSet returns an empty ToolSet.
func NewToolSet() *ToolSet {
	return &ToolSet{tools: make(map[string]Tool)}
}

// Register adds t to the set. Returns ErrNilTool if t is malformed
// (empty Name or nil Invoke), or ErrDuplicateTool if a tool with the same
// Name is already registered.
func (s *ToolSet) Register(t Tool) error {
	if err := t.validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tools[t.Name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateTool, t.Name)
	}
	s.tools[t.Name] = t
	return nil
}

// MustRegister is Register but panics on error. Intended for package-init
// wiring of a fixed tool set (e.g. in a downstream agent's constructor)
// where a registration failure indicates a programming error, not a
// runtime condition.
func (s *ToolSet) MustRegister(t Tool) {
	if err := s.Register(t); err != nil {
		panic(err)
	}
}

// Get returns the tool registered under name, or ErrToolNotFound.
func (s *ToolSet) Get(name string) (Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tools[name]
	if !ok {
		return Tool{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	return t, nil
}

// List returns every registered tool, sorted by Name for deterministic
// iteration (e.g. when rendering a model-facing tool manifest).
func (s *ToolSet) List() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Invoke looks up name and calls its Invoke function with args. A lookup
// failure returns ErrToolNotFound directly; a failure from the tool
// itself is wrapped in ErrToolInvocation so callers can distinguish "no
// such tool" from "the tool ran and failed".
func (s *ToolSet) Invoke(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	t, err := s.Get(name)
	if err != nil {
		return ToolResult{}, err
	}

	result, err := t.Invoke(ctx, args)
	if err != nil {
		return ToolResult{}, fmt.Errorf("%w: tool %q: %w", ErrToolInvocation, name, err)
	}
	return result, nil
}
