package tui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxRecentWorkerStatuses = 5

// WorkerStatusEntry is the process-agnostic worker status value rendered by the
// TUI.
type WorkerStatusEntry struct {
	ID    string
	CWD   string
	Label string
	State string
	Error string
}

type workerEventMsg struct{ entry WorkerStatusEntry }

// WorkerEventMsg wraps a worker status update as a Bubble Tea message.
func WorkerEventMsg(entry WorkerStatusEntry) tea.Msg { return workerEventMsg{entry: entry} }

func (m *Model) applyWorkerStatus(entry WorkerStatusEntry) {
	if entry.ID == "" {
		return
	}
	if m.workerStatuses == nil {
		m.workerStatuses = make(map[string]WorkerStatusEntry)
	}
	if _, ok := m.workerStatuses[entry.ID]; !ok {
		m.workerOrder = append(m.workerOrder, entry.ID)
	}
	m.workerStatuses[entry.ID] = entry
	m.pruneCompletedWorkers()
}

func (m *Model) pruneCompletedWorkers() {
	completed := 0
	for i := len(m.workerOrder) - 1; i >= 0; i-- {
		if workerStateTerminal(m.workerStatuses[m.workerOrder[i]].State) {
			completed++
			if completed > maxRecentWorkerStatuses {
				delete(m.workerStatuses, m.workerOrder[i])
				m.workerOrder = append(m.workerOrder[:i], m.workerOrder[i+1:]...)
			}
		}
	}
}

func (m Model) workerStatusEntries() []WorkerStatusEntry {
	var active []WorkerStatusEntry
	var completed []WorkerStatusEntry
	for _, id := range m.workerOrder {
		entry, ok := m.workerStatuses[id]
		if !ok {
			continue
		}
		if workerStateTerminal(entry.State) {
			completed = append(completed, entry)
		} else {
			active = append(active, entry)
		}
	}
	for i, j := 0, len(completed)-1; i < j; i, j = i+1, j-1 {
		completed[i], completed[j] = completed[j], completed[i]
	}
	return append(active, completed...)
}

func (m Model) hasWorkerStatuses() bool {
	for _, id := range m.workerOrder {
		if _, ok := m.workerStatuses[id]; ok {
			return true
		}
	}
	return false
}

func (m Model) screenChromeHeight() int {
	if m.hasWorkerStatuses() {
		return screenBaseChromeHeight + 1
	}
	return screenBaseChromeHeight
}

func (m Model) workerStatusStrip(w int) string {
	line := renderWorkerStatusStrip(m.workerStatusEntries(), w)
	return workerStripStyle.Render(line)
}

func renderWorkerStatusStrip(entries []WorkerStatusEntry, w int) string {
	if w < 1 {
		return ""
	}
	active, completed := splitWorkerEntries(entries)
	if len(active) == 0 && len(completed) == 0 {
		return ""
	}

	for labelMax := 12; labelMax >= 3; labelMax-- {
		if line, ok := tryWorkerLine(active, completed, labelMax, w); ok {
			return line
		}
	}
	for keepCompleted := len(completed) - 1; keepCompleted >= 0; keepCompleted-- {
		for labelMax := 12; labelMax >= 3; labelMax-- {
			if line, ok := tryWorkerLine(active, completed[:keepCompleted], labelMax, w); ok {
				return line
			}
		}
	}
	line, _ := tryWorkerLine(active, nil, 1, w)
	return fitWorkerLine(line, w)
}

func splitWorkerEntries(entries []WorkerStatusEntry) ([]WorkerStatusEntry, []WorkerStatusEntry) {
	var active []WorkerStatusEntry
	var completed []WorkerStatusEntry
	for _, entry := range entries {
		if workerStateTerminal(entry.State) {
			completed = append(completed, entry)
		} else {
			active = append(active, entry)
		}
	}
	return active, completed
}

func tryWorkerLine(active, completed []WorkerStatusEntry, labelMax, w int) (string, bool) {
	pieces := []string{"workers"}
	for _, entry := range append(append([]WorkerStatusEntry{}, active...), completed...) {
		pieces = append(pieces, workerPiece(entry, labelMax))
	}
	line := strings.Join(pieces, "  ")
	if lipgloss.Width(line) <= w {
		return line + strings.Repeat(" ", w-lipgloss.Width(line)), true
	}
	return line, false
}

func workerPiece(entry WorkerStatusEntry, labelMax int) string {
	label := strings.TrimSpace(entry.Label)
	if label == "" {
		label = filepath.Base(strings.TrimSpace(entry.CWD))
	}
	if label == "." || label == "/" || label == "" {
		label = "worker"
	}
	return clipWorkerText(label, labelMax) + "#" + shortWorkerID(entry.ID) + " " + workerStateWord(entry.State)
}

func shortWorkerID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 1 && id[0] == 'w' {
		return id[1:]
	}
	if len(id) > 4 {
		return id[len(id)-4:]
	}
	return id
}

func workerStateWord(state string) string {
	switch state {
	case "succeeded":
		return "done"
	case "timed_out":
		return "timeout"
	case "canceled":
		return "canceled"
	case "failed":
		return "failed"
	case "queued":
		return "queued"
	case "running":
		return "running"
	default:
		if strings.TrimSpace(state) == "" {
			return "unknown"
		}
		return state
	}
}

func workerStateTerminal(state string) bool {
	switch state {
	case "succeeded", "failed", "timed_out", "canceled":
		return true
	default:
		return false
	}
}

func fitWorkerLine(line string, w int) string {
	line = clipWorkerText(line, w)
	if width := lipgloss.Width(line); width < w {
		return line + strings.Repeat(" ", w-width)
	}
	return line
}

func clipWorkerText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if s == "" {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return string([]rune(s)[0])
	}
	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)+"~") > maxWidth {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "~"
	}
	return b.String() + "~"
}
