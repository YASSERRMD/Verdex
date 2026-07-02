package agentframework_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
)

func TestToolSet_Register_NilInvoke_ReturnsError(t *testing.T) {
	ts := agentframework.NewToolSet()
	err := ts.Register(agentframework.Tool{Name: "no-invoke"})
	if !errors.Is(err, agentframework.ErrNilTool) {
		t.Fatalf("Register() error = %v, want ErrNilTool", err)
	}
}

func TestToolSet_Register_EmptyName_ReturnsError(t *testing.T) {
	ts := agentframework.NewToolSet()
	err := ts.Register(agentframework.Tool{
		Invoke: func(context.Context, map[string]any) (agentframework.ToolResult, error) {
			return agentframework.ToolResult{}, nil
		},
	})
	if !errors.Is(err, agentframework.ErrNilTool) {
		t.Fatalf("Register() error = %v, want ErrNilTool", err)
	}
}

func TestToolSet_Register_Duplicate_ReturnsError(t *testing.T) {
	ts := agentframework.NewToolSet()
	tool := agentframework.Tool{
		Name: "echo",
		Invoke: func(_ context.Context, args map[string]any) (agentframework.ToolResult, error) {
			return agentframework.ToolResult{Content: "ok"}, nil
		},
	}
	if err := ts.Register(tool); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	err := ts.Register(tool)
	if !errors.Is(err, agentframework.ErrDuplicateTool) {
		t.Fatalf("Register() second call error = %v, want ErrDuplicateTool", err)
	}
}

func TestToolSet_Invoke_NotFound_ReturnsError(t *testing.T) {
	ts := agentframework.NewToolSet()
	_, err := ts.Invoke(context.Background(), "missing", nil)
	if !errors.Is(err, agentframework.ErrToolNotFound) {
		t.Fatalf("Invoke() error = %v, want ErrToolNotFound", err)
	}
}

func TestToolSet_Invoke_WrapsToolError(t *testing.T) {
	ts := agentframework.NewToolSet()
	sentinel := errors.New("boom")
	err := ts.Register(agentframework.Tool{
		Name: "failing",
		Invoke: func(context.Context, map[string]any) (agentframework.ToolResult, error) {
			return agentframework.ToolResult{}, sentinel
		},
	})
	if err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	_, err = ts.Invoke(context.Background(), "failing", nil)
	if !errors.Is(err, agentframework.ErrToolInvocation) {
		t.Fatalf("Invoke() error = %v, want wrapping ErrToolInvocation", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("Invoke() error = %v, want also wrapping the underlying sentinel", err)
	}
}

func TestToolSet_Invoke_Success(t *testing.T) {
	ts := agentframework.NewToolSet()
	err := ts.Register(agentframework.Tool{
		Name: "echo",
		Invoke: func(_ context.Context, args map[string]any) (agentframework.ToolResult, error) {
			return agentframework.ToolResult{Content: args["msg"].(string)}, nil //nolint:forcetypeassert // test fixture
		},
	})
	if err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	result, err := ts.Invoke(context.Background(), "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("Invoke() error = %v, want nil", err)
	}
	if result.Content != "hello" {
		t.Fatalf("Content = %q, want %q", result.Content, "hello")
	}
}

func TestToolSet_List_SortedByName(t *testing.T) {
	ts := agentframework.NewToolSet()
	noop := func(context.Context, map[string]any) (agentframework.ToolResult, error) {
		return agentframework.ToolResult{}, nil
	}
	ts.MustRegister(agentframework.Tool{Name: "zeta", Invoke: noop})
	ts.MustRegister(agentframework.Tool{Name: "alpha", Invoke: noop})
	ts.MustRegister(agentframework.Tool{Name: "mid", Invoke: noop})

	names := make([]string, 0, 3)
	for _, tool := range ts.List() {
		names = append(names, tool.Name)
	}
	want := []string{"alpha", "mid", "zeta"}
	if len(names) != len(want) {
		t.Fatalf("List() len = %d, want %d", len(names), len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("List()[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestToolSet_MustRegister_PanicsOnDuplicate(t *testing.T) {
	ts := agentframework.NewToolSet()
	noop := func(context.Context, map[string]any) (agentframework.ToolResult, error) {
		return agentframework.ToolResult{}, nil
	}
	ts.MustRegister(agentframework.Tool{Name: "one", Invoke: noop})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustRegister() did not panic on duplicate registration")
		}
	}()
	ts.MustRegister(agentframework.Tool{Name: "one", Invoke: noop})
}
