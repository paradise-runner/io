package claudeproc

import "testing"

func TestParseLine_Init(t *testing.T) {
	line := []byte(`{"type":"system","subtype":"init","session_id":"abc-123","model":"claude-opus-4-8"}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindInit {
		t.Fatalf("Kind = %v, want KindInit", ev.Kind)
	}
	if ev.SessionID != "abc-123" {
		t.Fatalf("SessionID = %q, want abc-123", ev.SessionID)
	}
	if ev.Model != "claude-opus-4-8" {
		t.Fatalf("Model = %q, want claude-opus-4-8", ev.Model)
	}
}

func TestParseLine_AssistantText(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindAssistantText {
		t.Fatalf("Kind = %v, want KindAssistantText", ev.Kind)
	}
	if ev.Text != "hello world" {
		t.Fatalf("Text = %q, want %q", ev.Text, "hello world")
	}
}

func TestParseLine_Result(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","session_id":"abc-123","is_error":false,"total_cost_usd":0.0123}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindResult {
		t.Fatalf("Kind = %v, want KindResult", ev.Kind)
	}
	if ev.SessionID != "abc-123" {
		t.Fatalf("SessionID = %q, want abc-123", ev.SessionID)
	}
	if ev.IsError {
		t.Fatalf("IsError = true, want false")
	}
	if ev.CostUSD != 0.0123 {
		t.Fatalf("CostUSD = %v, want 0.0123", ev.CostUSD)
	}
}

func TestParseLine_ResultUsage(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","session_id":"s","is_error":false,"total_cost_usd":0.05,"usage":{"input_tokens":7230,"output_tokens":4},"modelUsage":{"claude-sonnet-4-6":{"contextWindow":200000}}}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.InputTokens != 7230 {
		t.Fatalf("InputTokens = %d, want 7230", ev.InputTokens)
	}
	if ev.ContextWindow != 200000 {
		t.Fatalf("ContextWindow = %d, want 200000", ev.ContextWindow)
	}
}

func TestParseLine_UnknownType(t *testing.T) {
	line := []byte(`{"type":"stream_event","event":{"type":"content_block_delta"}}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindUnknown {
		t.Fatalf("Kind = %v, want KindUnknown", ev.Kind)
	}
}

func TestParseLine_InvalidJSON(t *testing.T) {
	_, err := ParseLine([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
