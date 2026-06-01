package codexproc

import (
	"encoding/json"

	"github.com/edward-champion/io/internal/claudeproc"
)

type rawLine struct {
	Type     string    `json:"type"`
	ThreadID string    `json:"thread_id"`
	Model    string    `json:"model"`
	Item     *rawItem  `json:"item"`
	Usage    *rawUsage `json:"usage"`
	Error    *rawError `json:"error"`
	Message  string    `json:"message"`
	Text     string    `json:"text"`
}

type rawItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Text   string `json:"text"`
}

type rawUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

type rawError struct {
	Message string `json:"message"`
}

// ParseLine parses one Codex JSONL event into io's normalized event shape.
// Unknown event types are intentionally ignored by returning KindUnknown.
func ParseLine(line []byte) (claudeproc.Event, error) {
	var r rawLine
	if err := json.Unmarshal(line, &r); err != nil {
		return claudeproc.Event{}, err
	}
	switch r.Type {
	case "thread.started":
		return claudeproc.Event{Kind: claudeproc.KindInit, SessionID: r.ThreadID, Model: r.Model}, nil
	case "item.completed":
		if r.Item != nil && r.Item.Type == "agent_message" {
			return claudeproc.Event{Kind: claudeproc.KindAssistantText, Text: r.Item.Text}, nil
		}
	case "turn.completed", "task_complete":
		ev := claudeproc.Event{Kind: claudeproc.KindResult, SessionID: r.ThreadID}
		if r.Usage != nil {
			ev.InputTokens = r.Usage.InputTokens
		}
		return ev, nil
	case "turn.failed", "error", "stream_error":
		return claudeproc.Event{Kind: claudeproc.KindResult, SessionID: r.ThreadID, IsError: true}, nil
	case "agent_message":
		if r.Message != "" {
			return claudeproc.Event{Kind: claudeproc.KindAssistantText, Text: r.Message}, nil
		}
		if r.Text != "" {
			return claudeproc.Event{Kind: claudeproc.KindAssistantText, Text: r.Text}, nil
		}
	case "token_count":
		ev := claudeproc.Event{Kind: claudeproc.KindResult, SessionID: r.ThreadID}
		if r.Usage != nil {
			ev.InputTokens = r.Usage.InputTokens
		}
		return ev, nil
	}
	return claudeproc.Event{Kind: claudeproc.KindUnknown}, nil
}
