package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestWorkerStatusStripNormalWidth(t *testing.T) {
	entries := []WorkerStatusEntry{
		{ID: "w1", Label: "docs", State: "running"},
		{ID: "w2", Label: "api", State: "succeeded"},
		{ID: "w3", Label: "sync", State: "failed"},
	}
	line := renderWorkerStatusStrip(entries, 72)
	for _, want := range []string{"workers", "docs#1 running", "api#2 done", "sync#3 failed"} {
		if !strings.Contains(line, want) {
			t.Fatalf("worker strip missing %q: %q", want, line)
		}
	}
	if got := lipgloss.Width(line); got != 72 {
		t.Fatalf("strip width = %d, want 72: %q", got, line)
	}
}

func TestWorkerStatusStripHiddenWhenEmpty(t *testing.T) {
	line := renderWorkerStatusStrip(nil, 40)
	if line != "" {
		t.Fatalf("empty worker strip = %q, want hidden", line)
	}

	m := newTestModel(&stubApp{})
	if m.hasWorkerStatuses() {
		t.Fatal("new model should not report worker statuses")
	}
	if m.screenChromeHeight() != screenBaseChromeHeight {
		t.Fatalf("screenChromeHeight = %d, want base %d", m.screenChromeHeight(), screenBaseChromeHeight)
	}
	if strings.Contains(m.View(), "workers idle") || strings.Contains(m.View(), "workers") {
		t.Fatalf("View should not render an idle worker row:\n%s", m.View())
	}
}

func TestWorkerStatusStripNarrowHidesCompletedBeforeActive(t *testing.T) {
	entries := []WorkerStatusEntry{
		{ID: "w1", Label: "frontend", State: "running"},
		{ID: "w2", Label: "docs", State: "succeeded"},
		{ID: "w3", Label: "api", State: "failed"},
	}
	line := renderWorkerStatusStrip(entries, 24)
	if !strings.Contains(line, "running") {
		t.Fatalf("narrow strip should keep active worker: %q", line)
	}
	if strings.Contains(line, "done") || strings.Contains(line, "failed") {
		t.Fatalf("narrow strip should hide completed workers first: %q", line)
	}
	if got := lipgloss.Width(line); got != 24 {
		t.Fatalf("strip width = %d, want 24: %q", got, line)
	}
}

func TestModelAppliesWorkerEvent(t *testing.T) {
	m := newTestModel(&stubApp{})
	beforeViewportH := m.messages.ViewportHeight()
	beforeChromeH := m.screenChromeHeight()
	updated, _ := m.Update(WorkerEventMsg(WorkerStatusEntry{ID: "w1", Label: "docs", State: "running"}))
	m = updated.(Model)

	if len(m.workerStatuses) != 1 {
		t.Fatalf("worker status count = %d, want 1", len(m.workerStatuses))
	}
	if !strings.Contains(m.workerStatusStrip(40), "docs#1 running") {
		t.Fatalf("worker strip missing running worker: %q", m.workerStatusStrip(40))
	}
	if m.screenChromeHeight() != beforeChromeH+1 {
		t.Fatalf("screenChromeHeight = %d, want %d", m.screenChromeHeight(), beforeChromeH+1)
	}
	if m.messages.ViewportHeight() != beforeViewportH-1 {
		t.Fatalf("viewport height = %d, want %d after worker row appears", m.messages.ViewportHeight(), beforeViewportH-1)
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	if !strings.Contains(m.View(), "docs#1 running") {
		t.Fatalf("View missing worker strip:\n%s", m.View())
	}
}
