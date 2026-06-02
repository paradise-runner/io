package dreamer

import (
	"testing"
	"time"
)

func TestCadenceRequiresThresholdAndIdle(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.Local)
	c := Cadence{Now: func() time.Time { return now }}
	st := State{ChatsSinceDream: 6, DreamChatThreshold: 7}
	if got := c.ShouldRun(st, IdleState{}); got != DecisionNone {
		t.Fatalf("decision below threshold = %s, want none", got)
	}
	st.ChatsSinceDream = 7
	for _, idle := range []IdleState{
		{UserTurnActive: true},
		{ActiveWorkers: 1},
		{Compacting: true},
		{Dreaming: true},
		{SetupRequired: true},
	} {
		if got := c.ShouldRun(st, idle); got != DecisionNone {
			t.Fatalf("decision for busy state %+v = %s, want none", idle, got)
		}
	}
	if got := c.ShouldRun(st, IdleState{}); got != DecisionDream {
		t.Fatalf("decision when idle = %s, want dream", got)
	}
}

func TestCadenceSuppressesSameDayDreams(t *testing.T) {
	now := time.Date(2026, 6, 2, 22, 0, 0, 0, time.Local)
	c := Cadence{Now: func() time.Time { return now }}
	st := State{
		LastDreamAt:        time.Date(2026, 6, 2, 9, 0, 0, 0, time.Local).Format(time.RFC3339),
		ChatsSinceDream:    10,
		DreamChatThreshold: 7,
	}
	if got := c.ShouldRun(st, IdleState{}); got != DecisionNone {
		t.Fatalf("decision = %s, want none after same-day dream", got)
	}
}

func TestCadenceMarkSucceededResetsCounter(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.Local)
	c := Cadence{Now: func() time.Time { return now }}
	st := State{ChatsSinceDream: 9}
	c.MarkSucceeded(&st)
	if st.ChatsSinceDream != 0 {
		t.Fatalf("ChatsSinceDream = %d, want 0", st.ChatsSinceDream)
	}
	if st.LastDreamAt != now.Format(time.RFC3339) {
		t.Fatalf("LastDreamAt = %q, want %q", st.LastDreamAt, now.Format(time.RFC3339))
	}
}
