package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/agentharness"
)

// settingsScreen edits harness settings + compaction threshold. Harness/model
// changes apply on the next session; the threshold is persisted immediately.
type settingsScreen struct {
	app        AppController
	harness    string
	model      string
	effort     string
	threshold  float64
	dreamChats int
	field      int // 0 = model, 1 = threshold, 2 = dreams, 3 = harness, 4 = effort
}

func newSettingsScreen(app AppController) *settingsScreen {
	st := app.Settings()
	st.Normalize()
	return &settingsScreen{
		app:        app,
		harness:    st.Harness,
		model:      st.ActiveModel(),
		effort:     st.ReasoningEffort,
		threshold:  st.CompactionThreshold,
		dreamChats: st.DreamChatThreshold,
	}
}

func (d *settingsScreen) Layout(width, height int) {}

func (d *settingsScreen) cycleModel(dir int) {
	settingsModels := agentharness.ModelOptions(d.harness)
	idx := 0
	for i, m := range settingsModels {
		if m == d.model {
			idx = i
		}
	}
	idx = (idx + dir + len(settingsModels)) % len(settingsModels)
	d.model = settingsModels[idx]
}

func (d *settingsScreen) cycleHarness(dir int) {
	harnesses := agentharness.HarnessOptions()
	idx := 0
	for i, h := range harnesses {
		if h == d.harness {
			idx = i
		}
	}
	idx = (idx + dir + len(harnesses)) % len(harnesses)
	d.harness = harnesses[idx]
	d.model = agentharness.DefaultModel(d.harness)
}

func (d *settingsScreen) cycleEffort(dir int) {
	efforts := agentharness.EffortOptions()
	idx := 0
	for i, effort := range efforts {
		if effort == d.effort {
			idx = i
		}
	}
	idx = (idx + dir + len(efforts)) % len(efforts)
	d.effort = efforts[idx]
}

func (d *settingsScreen) adjustThreshold(delta float64) {
	d.threshold += delta
	if d.threshold < 0.50 {
		d.threshold = 0.50
	}
	if d.threshold > 0.95 {
		d.threshold = 0.95
	}
}

func (d *settingsScreen) adjustDreamChats(delta int) {
	d.dreamChats += delta
	if d.dreamChats < 1 {
		d.dreamChats = 1
	}
	if d.dreamChats > 50 {
		d.dreamChats = 50
	}
}

func (d *settingsScreen) save() {
	_ = d.app.SetModel(d.model)
	st := d.app.Settings()
	st.SetHarness(d.harness)
	st.SetActiveModel(d.model)
	st.ReasoningEffort = d.effort
	st.CompactionThreshold = d.threshold
	st.DreamChatThreshold = d.dreamChats
	st.Normalize()
	_ = d.app.SaveSettings(st)
}

func (d *settingsScreen) Update(msg tea.Msg) (controlScreen, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}
	switch k.String() {
	case "esc":
		return nil, nil
	case "enter":
		d.save()
		return nil, nil
	case "up", "down", "tab":
		d.field = (d.field + 1) % 5
	case "left":
		switch d.field {
		case 0:
			d.cycleModel(-1)
		case 1:
			d.adjustThreshold(-0.05)
		case 2:
			d.adjustDreamChats(-1)
		case 3:
			d.cycleHarness(-1)
		case 4:
			d.cycleEffort(-1)
		}
	case "right":
		switch d.field {
		case 0:
			d.cycleModel(1)
		case 1:
			d.adjustThreshold(0.05)
		case 2:
			d.adjustDreamChats(1)
		case 3:
			d.cycleHarness(1)
		case 4:
			d.cycleEffort(1)
		}
	}
	return d, nil
}

func (d *settingsScreen) row(idx int, label, value string) string {
	marker := "  "
	if d.field == idx {
		marker = "▸ "
	}
	return fmt.Sprintf("%s%-12s ‹ %s ›", marker, label, value)
}

func (d *settingsScreen) View(width, height, frame int) string {
	body := d.row(0, "model", d.model) + "\n" +
		d.row(1, "context", fmt.Sprintf("compact at %.0f%%", d.threshold*100)) + "\n" +
		d.row(2, "dreams", fmt.Sprintf("after %d chats", d.dreamChats)) + "\n" +
		d.row(3, "harness", d.harness) + "\n" +
		d.row(4, "effort", d.effort)
	return renderControlPage("SETTINGS", "CONTROL", body, "up/down field   left/right change   enter save   esc back", width, height, frame)
}
