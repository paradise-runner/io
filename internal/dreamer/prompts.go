package dreamer

import (
	"fmt"
	"strings"

	"github.com/edward-champion/io/internal/chatlog"
)

const maxPromptHistory = 80

// BuildPrompt renders the model instruction for a dream pass.
func BuildPrompt(history []chatlog.Entry, existingMemory string) string {
	if len(history) > maxPromptHistory {
		history = history[len(history)-maxPromptHistory:]
	}
	var b strings.Builder
	b.WriteString("You are io's memory dreamer. Extract durable, user-relevant memories from the displayed chat transcript.\n")
	b.WriteString("Return only JSON in this shape: {\"candidates\":[{\"insight\":\"...\",\"evidence\":[\"role: short quote or event\"],\"confidence\":0.0,\"tags\":[\"preference\"]}]}.\n")
	b.WriteString("Skip trivia, one-off task state, secrets, credentials, and unsupported claims. Mark uncertain but useful inferences with a speculative tag.\n\n")
	b.WriteString("# Existing MEMORY.md\n")
	if strings.TrimSpace(existingMemory) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(strings.TrimSpace(existingMemory))
		b.WriteString("\n")
	}
	b.WriteString("\n# Recent displayed chat\n")
	for i, e := range history {
		role := strings.TrimSpace(e.Role)
		if role == "" {
			role = "unknown"
		}
		text := strings.TrimSpace(e.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "%d. %s: %s\n", i+1, role, text)
	}
	return b.String()
}
