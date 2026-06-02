package agentharness

import (
	"slices"
	"testing"

	"github.com/edward-champion/io/internal/claudeproc"
)

func TestClaudeStreamArgs(t *testing.T) {
	got := ClaudeStreamArgs(ClaudeStreamOptions{
		ResumeSessionID: "session-1",
		Model:           "opus",
		ReasoningEffort: "high",
		SoulPath:        "/tmp/SOUL.md",
		MemoryDir:       "/tmp/memory",
		MCPConfigPath:   "/tmp/mcp.json",
	})
	want := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
		"--resume", "session-1",
		"--model", "opus",
		"--effort", "high",
		"--append-system-prompt-file", "/tmp/SOUL.md",
		"--settings", `{"autoMemoryDirectory":"/tmp/memory"}`,
		"--mcp-config", "/tmp/mcp.json",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ClaudeStreamArgs = %#v, want %#v", got, want)
	}
}

func TestClaudePromptArgs(t *testing.T) {
	got := ClaudePromptArgs(ClaudePromptOptions{
		Model:           "sonnet",
		ReasoningEffort: "medium",
		SoulPath:        "/tmp/SOUL.md",
		MemoryDir:       "/tmp/memory",
	}, "summarize this")
	want := []string{
		"-p", "summarize this",
		"--output-format", "json",
		"--model", "sonnet",
		"--effort", "medium",
		"--append-system-prompt-file", "/tmp/SOUL.md",
		"--settings", `{"autoMemoryDirectory":"/tmp/memory"}`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ClaudePromptArgs = %#v, want %#v", got, want)
	}
}

func TestCodexExecArgsInitialTurn(t *testing.T) {
	got := CodexExecArgs(CodexExecOptions{
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		SoulPath:        "/tmp/SOUL.md",
		Workdir:         "/tmp/project",
		Prompt:          "hello",
	})
	want := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"--model", "gpt-5.4",
		"--config", `model_reasoning_effort="medium"`,
		"--config", `model_instructions_file="/tmp/SOUL.md"`,
		"--cd", "/tmp/project",
		"hello",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("CodexExecArgs initial = %#v, want %#v", got, want)
	}
}

func TestCodexExecArgsResume(t *testing.T) {
	got := CodexExecArgs(CodexExecOptions{
		ResumeSessionID: "thread-1",
		Model:           "gpt-5.5",
		ReasoningEffort: "high",
		Workdir:         "/tmp/project",
		Prompt:          "continue",
	})
	want := []string{
		"exec", "resume",
		"--json",
		"--skip-git-repo-check",
		"--model", "gpt-5.5",
		"--config", `model_reasoning_effort="high"`,
		"thread-1",
		"continue",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("CodexExecArgs resume = %#v, want %#v", got, want)
	}
}

func TestHarnessRegistryAliasesAndCapabilities(t *testing.T) {
	def, ok := DefinitionFor("claude_code")
	if !ok || def.Kind != Claude {
		t.Fatalf("DefinitionFor claude_code = %+v, %v; want Claude", def, ok)
	}
	mode, ok := InteractionModeFor("codex")
	if !ok || mode != InteractionExecTurns {
		t.Fatalf("InteractionModeFor codex = %q, %v; want exec turns", mode, ok)
	}
	if SupportsLiveCommands("codex") {
		t.Fatal("codex should not support live commands")
	}
}

func TestRuntimePromptCommandNormalizesHarnessModelAndEffort(t *testing.T) {
	rt := Runtime{
		ClaudePath:    "fakeclaude",
		SoulPath:      "/tmp/SOUL.md",
		MemoryDir:     "/tmp/memory",
		MCPConfigPath: "/tmp/mcp.json",
		Workdir:       "/tmp/root",
	}
	got, err := rt.PromptCommand(PromptRequest{
		Harness:         "claudecode",
		Model:           "gpt-5.4",
		ReasoningEffort: "turbo",
		Prompt:          "summarize",
	})
	if err != nil {
		t.Fatalf("PromptCommand error: %v", err)
	}
	wantArgs := ClaudePromptArgs(ClaudePromptOptions{
		Model:           DefaultClaudeModel,
		ReasoningEffort: DefaultReasoningEffort,
		SoulPath:        "/tmp/SOUL.md",
		MemoryDir:       "/tmp/memory",
		MCPConfigPath:   "/tmp/mcp.json",
	}, "summarize")
	if got.Harness != Claude || got.Bin != "fakeclaude" || got.CWD != "/tmp/root" || !slices.Equal(got.Args, wantArgs) {
		t.Fatalf("PromptCommand = %+v, want claude fakeclaude %#v cwd=/tmp/root", got, wantArgs)
	}
}

func TestRuntimeTurnCommandBuildsCodexResume(t *testing.T) {
	rt := Runtime{
		CodexPath: "fakecodex",
		SoulPath:  "/tmp/SOUL.md",
		Workdir:   "/tmp/root",
	}
	got, err := rt.TurnCommand(TurnRequest{
		Harness:         "codex",
		ResumeSessionID: "thread-1",
		Model:           "sonnet",
		ReasoningEffort: "high",
		Prompt:          "continue",
		CWD:             "/tmp/project",
	})
	if err != nil {
		t.Fatalf("TurnCommand error: %v", err)
	}
	wantArgs := CodexExecArgs(CodexExecOptions{
		ResumeSessionID: "thread-1",
		Model:           DefaultCodexModel,
		ReasoningEffort: "high",
		SoulPath:        "/tmp/SOUL.md",
		Workdir:         "/tmp/project",
		Prompt:          "continue",
	})
	if got.Harness != Codex || got.Bin != "fakecodex" || got.CWD != "/tmp/project" || !slices.Equal(got.Args, wantArgs) {
		t.Fatalf("TurnCommand = %+v, want codex fakecodex %#v cwd=/tmp/project", got, wantArgs)
	}
}

func TestParseLineDispatchesByHarness(t *testing.T) {
	ev, err := ParseLine("codex", []byte(`{"type":"thread.started","thread_id":"t1","model":"gpt-5.4"}`))
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != claudeproc.KindInit || ev.SessionID != "t1" {
		t.Fatalf("ParseLine event = %+v, want codex init", ev)
	}
}
