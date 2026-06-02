package compaction

import (
	"testing"

	"github.com/edward-champion/io/internal/claudeproc"
)

func result(input, window int, session string) claudeproc.Event {
	return claudeproc.Event{
		Kind:          claudeproc.KindResult,
		SessionID:     session,
		InputTokens:   input,
		ContextWindow: window,
	}
}

func TestControllerBelowThresholdDoesNothing(t *testing.T) {
	c := Controller{Threshold: 0.75}
	if got := c.Observe(result(749, 1000, "s1"), ""); got != DecisionNone {
		t.Fatalf("decision = %s, want none", got)
	}
}

func TestControllerAboveThresholdCompactsOnce(t *testing.T) {
	c := Controller{Threshold: 0.75}
	if got := c.Observe(result(750, 1000, "s1"), ""); got != DecisionCompact {
		t.Fatalf("decision = %s, want compact", got)
	}
	if got := c.Observe(result(750, 1000, "s1"), ""); got != DecisionNone {
		t.Fatalf("repeat decision = %s, want none", got)
	}
	if got := c.Observe(result(740, 1000, "s1"), ""); got != DecisionNone {
		t.Fatalf("lower-token decision = %s, want none", got)
	}
}

func TestControllerCompactsAgainAfterUsageGrows(t *testing.T) {
	c := Controller{Threshold: 0.75}
	if got := c.Observe(result(750, 1000, "s1"), ""); got != DecisionCompact {
		t.Fatalf("decision = %s, want compact", got)
	}
	if got := c.Observe(result(900, 1000, "s1"), ""); got != DecisionCompact {
		t.Fatalf("higher-token decision = %s, want compact", got)
	}
}

func TestControllerIgnoresMissingUsage(t *testing.T) {
	c := Controller{Threshold: 0.75}
	for _, ev := range []claudeproc.Event{
		result(0, 1000, "s1"),
		result(750, 0, "s1"),
		{Kind: claudeproc.KindAssistantText, Text: "hello"},
	} {
		if got := c.Observe(ev, ""); got != DecisionNone {
			t.Fatalf("decision for %+v = %s, want none", ev, got)
		}
	}
}

func TestControllerResetsForNewSession(t *testing.T) {
	c := Controller{Threshold: 0.75}
	if got := c.Observe(result(800, 1000, "s1"), ""); got != DecisionCompact {
		t.Fatalf("decision = %s, want compact", got)
	}
	if got := c.Observe(result(800, 1000, "s2"), ""); got != DecisionCompact {
		t.Fatalf("new-session decision = %s, want compact", got)
	}
}

func TestControllerUsesFallbackSessionID(t *testing.T) {
	c := Controller{Threshold: 0.75}
	if got := c.Observe(result(800, 1000, ""), "s1"); got != DecisionCompact {
		t.Fatalf("decision = %s, want compact", got)
	}
	if got := c.Observe(result(800, 1000, ""), "s1"); got != DecisionNone {
		t.Fatalf("repeat decision = %s, want none", got)
	}
}

func TestControllerInvalidThresholdUsesDefault(t *testing.T) {
	c := Controller{Threshold: 2}
	if got := c.Observe(result(740, 1000, "s1"), ""); got != DecisionNone {
		t.Fatalf("decision at 74%% = %s, want none", got)
	}
	if got := c.Observe(result(750, 1000, "s1"), ""); got != DecisionCompact {
		t.Fatalf("decision at default threshold = %s, want compact", got)
	}
}
