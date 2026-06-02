package dreamer

import "time"

// Cadence decides whether a dream pass should start.
type Cadence struct {
	Now func() time.Time
}

// Decision is the cadence gate's recommendation.
type Decision string

const (
	DecisionNone  Decision = "none"
	DecisionDream Decision = "dream"
)

// State is the persisted dream bookkeeping needed by the cadence gate.
type State struct {
	LastDreamAt        string
	ChatsSinceDream    int
	DreamChatThreshold int
}

// IdleState captures app activity that must be quiet before dreaming.
type IdleState struct {
	UserTurnActive bool
	ActiveWorkers  int
	Compacting     bool
	Dreaming       bool
	SetupRequired  bool
}

// ShouldRun returns DecisionDream only when enough chats have completed, the
// app is idle, and no dream has already run today.
func (c Cadence) ShouldRun(st State, idle IdleState) Decision {
	if idle.UserTurnActive || idle.ActiveWorkers > 0 || idle.Compacting || idle.Dreaming || idle.SetupRequired {
		return DecisionNone
	}
	threshold := st.DreamChatThreshold
	if threshold <= 0 {
		threshold = 7
	}
	if st.ChatsSinceDream < threshold {
		return DecisionNone
	}
	if dreamedToday(st.LastDreamAt, c.now()) {
		return DecisionNone
	}
	return DecisionDream
}

// MarkSucceeded stamps a successful dream and resets the post-dream chat count.
func (c Cadence) MarkSucceeded(st *State) {
	st.LastDreamAt = c.now().Format(time.RFC3339)
	st.ChatsSinceDream = 0
}

func (c Cadence) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func dreamedToday(last string, now time.Time) bool {
	if last == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return false
	}
	t = t.In(now.Location())
	return t.Year() == now.Year() && t.YearDay() == now.YearDay()
}
