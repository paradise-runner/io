package claudeproc

import (
	"encoding/json"
	"strings"
)

type rawLine struct {
	Type       string                   `json:"type"`
	Subtype    string                   `json:"subtype"`
	SessionID  string                   `json:"session_id"`
	Model      string                   `json:"model"`
	IsError    bool                     `json:"is_error"`
	CostUSD    float64                  `json:"total_cost_usd"`
	Message    *rawMessage              `json:"message"`
	Usage      *rawUsage                `json:"usage"`
	ModelUsage map[string]rawModelUsage `json:"modelUsage"`
}

type rawUsage struct {
	InputTokens int `json:"input_tokens"`
}

type rawModelUsage struct {
	ContextWindow int `json:"contextWindow"`
}

type rawMessage struct {
	Role    string       `json:"role"`
	Content []rawContent `json:"content"`
}

type rawContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseLine parses a single line of claude stream-json output into an Event.
// Lines whose type we do not consume return an Event with Kind == KindUnknown
// and a nil error. Malformed JSON returns a non-nil error.
func ParseLine(line []byte) (Event, error) {
	var r rawLine
	if err := json.Unmarshal(line, &r); err != nil {
		return Event{}, err
	}
	switch r.Type {
	case "system":
		if r.Subtype == "init" {
			return Event{Kind: KindInit, SessionID: r.SessionID, Model: r.Model}, nil
		}
	case "assistant":
		if r.Message != nil {
			return Event{Kind: KindAssistantText, Text: joinText(r.Message.Content)}, nil
		}
	case "result":
		ev := Event{Kind: KindResult, SessionID: r.SessionID, IsError: r.IsError, CostUSD: r.CostUSD}
		if r.Usage != nil {
			ev.InputTokens = r.Usage.InputTokens
		}
		for _, mu := range r.ModelUsage {
			if mu.ContextWindow > 0 {
				ev.ContextWindow = mu.ContextWindow
				break
			}
		}
		return ev, nil
	}
	return Event{Kind: KindUnknown}, nil
}

func joinText(cs []rawContent) string {
	var b strings.Builder
	for _, c := range cs {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}
