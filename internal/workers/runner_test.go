package workers

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/edward-champion/io/internal/agentharness"
)

func TestClaudeRunnerCommandAndJSONParse(t *testing.T) {
	var gotBin, gotCWD string
	var gotArgs []string
	r := ClaudeRunner{
		ClaudePath:    "fakeclaude",
		SoulPath:      "/tmp/SOUL.md",
		MemoryDir:     "/tmp/memory",
		MCPConfigPath: "/tmp/mcp.json",
		RunCommand: func(ctx context.Context, bin string, args []string, cwd string) ([]byte, error) {
			gotBin, gotArgs, gotCWD = bin, args, cwd
			return []byte(`{"result":"worker summary"}`), nil
		},
	}

	result, err := r.Run(context.Background(), Request{
		CWD:             "/tmp/project",
		Task:            "summarize",
		Model:           "opus",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Summary != "worker summary" {
		t.Fatalf("Summary = %q, want worker summary", result.Summary)
	}
	wantArgs := agentharness.ClaudePromptArgs(agentharness.ClaudePromptOptions{
		Model:           "opus",
		ReasoningEffort: "high",
		SoulPath:        "/tmp/SOUL.md",
		MemoryDir:       "/tmp/memory",
		MCPConfigPath:   "/tmp/mcp.json",
	}, "summarize")
	if gotBin != "fakeclaude" || gotCWD != "/tmp/project" || !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("command = %q %#v cwd=%q, want fakeclaude %#v cwd=/tmp/project", gotBin, gotArgs, gotCWD, wantArgs)
	}
}

func TestClaudeRunnerParsesStreamJSON(t *testing.T) {
	out := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"part one"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"part two"}]}}
{"type":"result","subtype":"success","is_error":false}`)
	result := parseClaudeWorkerOutput(out)
	if result.Summary != "part one\npart two" {
		t.Fatalf("Summary = %q, want joined assistant text", result.Summary)
	}
}

func TestClaudeRunnerStructuredFailure(t *testing.T) {
	r := ClaudeRunner{
		RunCommand: func(ctx context.Context, bin string, args []string, cwd string) ([]byte, error) {
			return []byte(`{"error":"permission denied","is_error":true}`), errors.New("exit status 1")
		},
	}
	result, err := r.Run(context.Background(), Request{Task: "x"})
	if err == nil {
		t.Fatal("Run error = nil, want command error")
	}
	if result.Error != "permission denied" {
		t.Fatalf("Error = %q, want permission denied", result.Error)
	}
}

func TestCodexRunnerCommandAndParse(t *testing.T) {
	var gotBin, gotCWD string
	var gotArgs []string
	r := CodexRunner{
		CodexPath: "fakecodex",
		SoulPath:  "/tmp/SOUL.md",
		RunCommand: func(ctx context.Context, bin string, args []string, cwd string) ([]byte, error) {
			gotBin, gotArgs, gotCWD = bin, args, cwd
			return []byte(`{"type":"thread.started","thread_id":"t1","model":"gpt-5.4"}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
{"type":"turn.completed","thread_id":"t1"}`), nil
		},
	}

	result, err := r.Run(context.Background(), Request{
		CWD:             "/tmp/project",
		Task:            "build",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Summary != "done" {
		t.Fatalf("Summary = %q, want done", result.Summary)
	}
	wantArgs := agentharness.CodexExecArgs(agentharness.CodexExecOptions{
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		SoulPath:        "/tmp/SOUL.md",
		Workdir:         "/tmp/project",
		Prompt:          "build",
	})
	if gotBin != "fakecodex" || gotCWD != "/tmp/project" || !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("command = %q %#v cwd=%q, want fakecodex %#v cwd=/tmp/project", gotBin, gotArgs, gotCWD, wantArgs)
	}
}

func TestCodexRunnerFailureParse(t *testing.T) {
	result := parseCodexWorkerOutput([]byte(`{"type":"turn.failed","thread_id":"t1"}`))
	if result.Error != "codex worker failed" {
		t.Fatalf("Error = %q, want codex worker failed", result.Error)
	}
}

func TestHarnessRunnerDispatches(t *testing.T) {
	var gotBin string
	r := HarnessRunner{
		Runtime: agentharness.Runtime{
			CodexPath: "fakecodex",
			RunCommand: func(ctx context.Context, bin string, args []string, cwd string) ([]byte, error) {
				gotBin = bin
				return []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"codex"}}
{"type":"turn.completed","thread_id":"t1"}`), nil
			},
		},
	}
	result, err := r.Run(context.Background(), Request{Harness: "codex", Task: "x"})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if gotBin != "fakecodex" || result.Summary != "codex" {
		t.Fatalf("dispatch bin=%q result=%+v, want fakecodex/codex", gotBin, result)
	}
}
