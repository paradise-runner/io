package persona

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/edward-champion/io/internal/claudeproc"
)

// buildFakeClaude compiles the fake claude binary once per test run.
func buildFakeClaude(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fakeclaude")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/fakeclaude/")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("building fake claude: %v", err)
	}
	return bin
}

func buildFakeCodex(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fakecodex")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/fakecodex/")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("building fake codex: %v", err)
	}
	return bin
}

func waitFor(t *testing.T, ch <-chan claudeproc.Event, kind claudeproc.EventKind) claudeproc.Event {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Kind == kind {
				return ev
			}
		case <-timeout:
			t.Fatalf("timed out waiting for event kind %v", kind)
		}
	}
}

func TestPersona_CapturesSessionIDOnStart(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.SessionID != "fake-session" {
		t.Fatalf("init SessionID = %q, want fake-session", init.SessionID)
	}
	if p.SessionID() != "fake-session" {
		t.Fatalf("SessionID() = %q, want fake-session", p.SessionID())
	}
}

func TestPersona_ResumePassesSessionID(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake, ResumeSessionID: "prior-session"})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.SessionID != "prior-session" {
		t.Fatalf("resumed SessionID = %q, want prior-session", init.SessionID)
	}
}

func TestPersona_ClaudePassesModelAndEffort(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake, Model: "opus", ReasoningEffort: "high"})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.Model != "opus/high" {
		t.Fatalf("init Model = %q, want opus/high", init.Model)
	}
}

func TestPersona_SendReturnsAssistantText(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	waitFor(t, p.Events(), claudeproc.KindInit)

	if err := p.Send("hello io"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	got := waitFor(t, p.Events(), claudeproc.KindAssistantText)
	if got.Text != "echo: hello io" {
		t.Fatalf("assistant Text = %q, want %q", got.Text, "echo: hello io")
	}
}

func TestPersona_CodexDefaultsModelAndEffort(t *testing.T) {
	fake := buildFakeCodex(t)
	p, err := New(Config{Harness: "codex", CodexPath: fake})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	if err := p.Send("hello io"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.SessionID != "fake-codex-session" {
		t.Fatalf("codex SessionID = %q, want fake-codex-session", init.SessionID)
	}
	got := waitFor(t, p.Events(), claudeproc.KindAssistantText)
	want := "model=gpt-5.4 effort=medium echo: hello io"
	if got.Text != want {
		t.Fatalf("assistant Text = %q, want %q", got.Text, want)
	}
}

func TestPersona_CodexResumePassesSessionID(t *testing.T) {
	fake := buildFakeCodex(t)
	p, err := New(Config{Harness: "codex", CodexPath: fake, ResumeSessionID: "prior-codex"})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	if err := p.Send("hello io"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.SessionID != "prior-codex" {
		t.Fatalf("codex resumed SessionID = %q, want prior-codex", init.SessionID)
	}
}
