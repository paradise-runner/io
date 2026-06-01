package claudeproc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeUserTurn(t *testing.T) {
	got, err := EncodeUserTurn("hello io")
	if err != nil {
		t.Fatalf("EncodeUserTurn error: %v", err)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Fatalf("encoded line must end with newline, got %q", got)
	}

	// It must be a single line of valid JSON with the expected shape.
	var decoded struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(got))), &decoded); err != nil {
		t.Fatalf("encoded line is not valid JSON: %v", err)
	}
	if decoded.Type != "user" {
		t.Fatalf("Type = %q, want user", decoded.Type)
	}
	if decoded.Message.Role != "user" {
		t.Fatalf("Role = %q, want user", decoded.Message.Role)
	}
	if len(decoded.Message.Content) != 1 || decoded.Message.Content[0].Text != "hello io" {
		t.Fatalf("Content = %+v, want one text block 'hello io'", decoded.Message.Content)
	}
}

func TestEncodeUserTurn_NoEmbeddedNewline(t *testing.T) {
	got, err := EncodeUserTurn("line1\nline2")
	if err != nil {
		t.Fatalf("EncodeUserTurn error: %v", err)
	}
	// Exactly one trailing newline; the embedded newline must be JSON-escaped,
	// not a literal byte that would split the stdin line.
	if strings.Count(string(got), "\n") != 1 {
		t.Fatalf("expected exactly one newline (the line terminator), got %q", got)
	}
}
