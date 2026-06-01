//go:build integration

package persona

import (
	"testing"
	"time"

	"github.com/edward-champion/io/internal/claudeproc"
)

// TestPersona_RealClaude_Smoke runs one turn against the real `claude` binary
// and asserts we capture a session id and receive assistant text. Requires a
// working `claude` install and auth on PATH.
func TestPersona_RealClaude_Smoke(t *testing.T) {
	p, err := New(Config{}) // ClaudePath empty -> "claude" on PATH
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	deadline := time.After(60 * time.Second)
	gotInit := false
	gotText := false

	if err := p.Send("Reply with exactly: pong"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	for !(gotInit && gotText) {
		select {
		case ev, ok := <-p.Events():
			if !ok {
				t.Fatal("event channel closed before smoke assertions satisfied")
			}
			switch ev.Kind {
			case claudeproc.KindInit:
				if ev.SessionID == "" {
					t.Fatal("init event had empty session id")
				}
				gotInit = true
			case claudeproc.KindAssistantText:
				if ev.Text != "" {
					gotText = true
				}
			}
		case <-deadline:
			t.Fatalf("timed out (init=%v text=%v)", gotInit, gotText)
		}
	}
}
