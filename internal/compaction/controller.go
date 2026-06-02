// Package compaction decides when io should ask the live persona to compact
// its context. It does not know how commands are sent.
package compaction

import "github.com/edward-champion/io/internal/claudeproc"

const defaultThreshold = 0.75

// Decision is the controller's recommendation after observing an event.
type Decision string

const (
	DecisionNone    Decision = "none"
	DecisionCompact Decision = "compact"
)

// Controller tracks context usage for one persona session.
type Controller struct {
	Threshold float64

	lastSession string
	compactedAt int
}

// Observe consumes a normalized persona event and returns whether compaction
// should be requested. sessionID is used when a result event omits its session.
func (c *Controller) Observe(ev claudeproc.Event, sessionID string) Decision {
	if ev.Kind != claudeproc.KindResult {
		return DecisionNone
	}
	sid := ev.SessionID
	if sid == "" {
		sid = sessionID
	}
	if sid != c.lastSession {
		c.lastSession = sid
		c.compactedAt = 0
	}
	if ev.InputTokens <= 0 || ev.ContextWindow <= 0 {
		return DecisionNone
	}
	used := float64(ev.InputTokens) / float64(ev.ContextWindow)
	if used < c.threshold() {
		return DecisionNone
	}
	if ev.InputTokens <= c.compactedAt {
		return DecisionNone
	}
	c.compactedAt = ev.InputTokens
	return DecisionCompact
}

// Reset forgets prior compaction decisions, usually after starting a new
// persona session or changing settings.
func (c *Controller) Reset() {
	c.lastSession = ""
	c.compactedAt = 0
}

func (c Controller) threshold() float64 {
	if c.Threshold <= 0 || c.Threshold > 1 {
		return defaultThreshold
	}
	return c.Threshold
}
