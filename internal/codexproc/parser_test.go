package codexproc

import (
	"testing"

	"github.com/edward-champion/io/internal/claudeproc"
)

func TestParseThreadStarted(t *testing.T) {
	ev, err := ParseLine([]byte(`{"type":"thread.started","thread_id":"0199a213-81c0-7800-8aa1-bbab2a035a53","model":"gpt-5.4"}`))
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != claudeproc.KindInit {
		t.Fatalf("Kind = %v, want init", ev.Kind)
	}
	if ev.SessionID != "0199a213-81c0-7800-8aa1-bbab2a035a53" {
		t.Fatalf("SessionID = %q", ev.SessionID)
	}
	if ev.Model != "gpt-5.4" {
		t.Fatalf("Model = %q", ev.Model)
	}
}

func TestParseAgentMessageItem(t *testing.T) {
	ev, err := ParseLine([]byte(`{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Repo contains docs."}}`))
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != claudeproc.KindAssistantText {
		t.Fatalf("Kind = %v, want assistant text", ev.Kind)
	}
	if ev.Text != "Repo contains docs." {
		t.Fatalf("Text = %q", ev.Text)
	}
}

func TestParseTurnCompletedUsage(t *testing.T) {
	ev, err := ParseLine([]byte(`{"type":"turn.completed","usage":{"input_tokens":24763,"cached_input_tokens":24448,"output_tokens":122,"reasoning_output_tokens":0}}`))
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != claudeproc.KindResult {
		t.Fatalf("Kind = %v, want result", ev.Kind)
	}
	if ev.InputTokens != 24763 {
		t.Fatalf("InputTokens = %d", ev.InputTokens)
	}
}
