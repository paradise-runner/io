package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/edward-champion/io/internal/chatlog"
	"github.com/edward-champion/io/internal/dreamer"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/workers"
)

func TestNewApp_MissingSoulDefersPersonaStart(t *testing.T) {
	root := t.TempDir()
	a, err := newApp(root, personastate.State{}, runtimeConfig{})
	if err != nil {
		t.Fatalf("newApp error: %v", err)
	}
	defer a.Close()
	if !a.NeedsSetup() {
		t.Fatal("NeedsSetup = false, want true without SOUL.md")
	}
	if a.p != nil {
		t.Fatal("persona should not start before setup")
	}
}

func TestNewAppWritesMCPConfig(t *testing.T) {
	root := t.TempDir()
	a, err := newApp(root, personastate.State{}, runtimeConfig{})
	if err != nil {
		t.Fatalf("newApp error: %v", err)
	}
	defer a.Close()

	b, err := os.ReadFile(a.mcpConfigPath)
	if err != nil {
		t.Fatalf("read MCP config: %v", err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("decode MCP config: %v", err)
	}
	ioServer, ok := cfg.MCPServers["io"]
	if !ok {
		t.Fatalf("MCP config missing io server: %s", b)
	}
	if len(ioServer.Args) != 3 || ioServer.Args[0] != "mcp-stdio" || ioServer.Args[1] != "--control-socket" {
		t.Fatalf("MCP args = %v, want mcp-stdio --control-socket <socket>", ioServer.Args)
	}
	if !strings.HasSuffix(ioServer.Args[2], "io.sock") {
		t.Fatalf("control socket arg = %q, want io.sock", ioServer.Args[2])
	}
}

func TestNeedsSetup_FalseWhenSoulExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(personastate.SoulPath(root), []byte("personality"), 0o600); err != nil {
		t.Fatalf("write soul: %v", err)
	}
	got, err := needsSetup(root)
	if err != nil {
		t.Fatalf("needsSetup error: %v", err)
	}
	if got {
		t.Fatal("needsSetup = true, want false with SOUL.md")
	}
}

func TestSpawnWorkerAppliesAppDefaults(t *testing.T) {
	got := make(chan workers.Request, 1)
	a := &app{
		root: t.TempDir(),
		st: personastate.State{
			Harness:         "codex",
			CodexModel:      "gpt-5.4",
			ReasoningEffort: "high",
		},
		workers: workers.NewManager(workers.RunnerFunc(func(ctx context.Context, req workers.Request) (workers.Result, error) {
			got <- req
			return workers.Result{Summary: "ok"}, nil
		})),
	}

	if _, err := a.spawnWorker(context.Background(), workers.Request{Task: "inspect"}); err != nil {
		t.Fatalf("spawnWorker error: %v", err)
	}
	select {
	case req := <-got:
		if req.CWD != a.root {
			t.Fatalf("CWD = %q, want root %q", req.CWD, a.root)
		}
		if req.Harness != "codex" || req.Model != "gpt-5.4" || req.ReasoningEffort != "high" {
			t.Fatalf("request harness/model/effort = %+v, want codex/gpt-5.4/high", req)
		}
		if req.Timeout != defaultWorkerTimeout {
			t.Fatalf("Timeout = %s, want %s", req.Timeout, defaultWorkerTimeout)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker request")
	}
}

func TestWorkerInjectionText(t *testing.T) {
	text := workerInjectionText(workers.Event{
		Status:  workers.Status{ID: "w1", CWD: "/tmp/project", State: workers.StateSucceeded},
		Request: workers.Request{Task: "summarize docs"},
		Result:  &workers.Result{Summary: "found three notes"},
	})
	for _, want := range []string{
		"Worker w1 finished in /tmp/project.",
		"Task:\nsummarize docs",
		"Result:\nfound three notes",
		"Please fold this into the conversation briefly.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("workerInjectionText missing %q:\n%s", want, text)
		}
	}
}

func TestPrepareDreamStartsOnlyWhenIdleAndPastThreshold(t *testing.T) {
	a := &app{
		root: t.TempDir(),
		st: personastate.State{
			Harness:            "claude",
			Model:              "sonnet",
			DreamChatThreshold: 7,
			ChatsSinceDream:    7,
		},
		workers:      workers.NewManager(workers.RunnerFunc(func(context.Context, workers.Request) (workers.Result, error) { return workers.Result{}, nil })),
		dreamCadence: dreamer.Cadence{Now: func() time.Time { return time.Date(2026, 6, 2, 10, 0, 0, 0, time.Local) }},
	}
	a.mu.Lock()
	job, ok := a.prepareDreamLocked()
	a.mu.Unlock()
	if !ok {
		t.Fatal("prepareDreamLocked should start when idle and past threshold")
	}
	if !a.dreaming {
		t.Fatal("prepareDreamLocked should mark dreaming")
	}
	if job.chatsAtStart != 7 {
		t.Fatalf("chatsAtStart = %d, want 7", job.chatsAtStart)
	}

	a.dreaming = false
	a.userTurnActive = true
	a.mu.Lock()
	_, ok = a.prepareDreamLocked()
	a.mu.Unlock()
	if ok {
		t.Fatal("prepareDreamLocked should not start during active user turn")
	}
}

func TestExecuteDreamWritesMemory(t *testing.T) {
	root := t.TempDir()
	if err := chatlog.Append(personastate.HistoryPath(root), chatlog.Entry{Role: "you", Text: "please keep answers short"}); err != nil {
		t.Fatalf("append history: %v", err)
	}
	a := &app{}
	err := a.executeDream(context.Background(), dreamJob{
		root: root,
		state: personastate.State{
			Harness:         "claude",
			ClaudeModel:     "sonnet",
			ReasoningEffort: "medium",
			DreamerModel:    "sonnet",
		},
		runner: dreamer.Runner{RunCommand: func(context.Context, string, []string, string) ([]byte, error) {
			return []byte(`{"candidates":[{"insight":"User prefers concise answers","evidence":["you: please keep answers short"],"confidence":0.9,"tags":["preference"]}]}`), nil
		}},
	})
	if err != nil {
		t.Fatalf("executeDream error: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(personastate.MemoryDir(root), "MEMORY.md"))
	if err != nil {
		t.Fatalf("read memory: %v", err)
	}
	if !strings.Contains(string(b), "User prefers concise answers") {
		t.Fatalf("memory missing dreamed insight:\n%s", b)
	}
}

func TestRunDreamSuccessUpdatesState(t *testing.T) {
	root := t.TempDir()
	if err := chatlog.Append(personastate.HistoryPath(root), chatlog.Entry{Role: "you", Text: "remember I use tmux"}); err != nil {
		t.Fatalf("append history: %v", err)
	}
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.Local)
	a := &app{
		root: root,
		st: personastate.State{
			Harness:            "claude",
			ClaudeModel:        "sonnet",
			ReasoningEffort:    "medium",
			DreamChatThreshold: 7,
			ChatsSinceDream:    8,
			DreamerModel:       "sonnet",
		},
		dreaming:     true,
		dreamCadence: dreamer.Cadence{Now: func() time.Time { return now }},
	}
	a.runDream(dreamJob{
		root:         root,
		state:        a.st,
		chatsAtStart: 7,
		runner: dreamer.Runner{RunCommand: func(context.Context, string, []string, string) ([]byte, error) {
			return []byte(`{"candidates":[{"insight":"User uses tmux","confidence":0.8}]}`), nil
		}},
	})
	if a.dreaming {
		t.Fatal("runDream should clear dreaming")
	}
	if a.st.LastDreamAt != now.Format(time.RFC3339) {
		t.Fatalf("LastDreamAt = %q, want %q", a.st.LastDreamAt, now.Format(time.RFC3339))
	}
	if a.st.ChatsSinceDream != 1 {
		t.Fatalf("ChatsSinceDream = %d, want 1", a.st.ChatsSinceDream)
	}
}
