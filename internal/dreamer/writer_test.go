package dreamer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderMemoryAppendsSections(t *testing.T) {
	decisions := []CurationDecision{
		{Action: ActionWrite, Candidate: Candidate{Insight: "User likes terminal UIs", Evidence: []string{"you: tui"}, Confidence: 0.8, Tags: []string{"preference"}}},
		{Action: ActionMerge, Candidate: Candidate{Insight: "User prefers concise summaries", Evidence: []string{"you: short"}, Confidence: 0.9}},
		{Action: ActionUpdate, Candidate: Candidate{Insight: "User changed model to opus", Confidence: 0.7}},
		{Action: ActionSkip, Candidate: Candidate{Insight: "skip me"}},
	}
	got := RenderMemory("", decisions)
	for _, want := range []string{"# io Memory", "## Durable Notes", "User likes terminal UIs", "## Evidence Updates", "## Updates"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered memory missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "skip me") {
		t.Fatalf("rendered memory included skipped candidate:\n%s", got)
	}
}

func TestWriteAtomicWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory", "MEMORY.md")
	err := WriteAtomic(path, "", []CurationDecision{
		{Action: ActionWrite, Candidate: Candidate{Insight: "User likes terminal UIs", Confidence: 0.8}},
	})
	if err != nil {
		t.Fatalf("WriteAtomic error: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read memory: %v", err)
	}
	if !strings.Contains(string(b), "User likes terminal UIs") {
		t.Fatalf("memory missing insight:\n%s", b)
	}
}
