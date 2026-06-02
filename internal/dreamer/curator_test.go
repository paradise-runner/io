package dreamer

import "testing"

func TestCurateWriteMergeUpdateSkipAndDuplicates(t *testing.T) {
	existing := "# io Memory\n\n- User prefers concise summaries\n  - Evidence: old"
	candidates := []Candidate{
		{Insight: "User likes terminal UIs", Confidence: 0.8},
		{Insight: "User prefers concise summaries", Evidence: []string{"new evidence"}, Confidence: 0.9},
		{Insight: "User changed default model to opus", Confidence: 0.8, Tags: []string{"update"}},
		{Insight: "User likes terminal UIs", Confidence: 0.8},
		{Insight: "Unsupported maybe", Confidence: 0.2},
		{Insight: "This contradicts prior memory", Confidence: 0.9, Tags: []string{"contradiction"}},
	}
	got := Curate(existing, candidates)
	want := []Action{ActionWrite, ActionMerge, ActionUpdate, ActionSkip, ActionSkip, ActionSkip}
	if len(got) != len(want) {
		t.Fatalf("decisions len = %d, want %d", len(got), len(want))
	}
	for i, action := range want {
		if got[i].Action != action {
			t.Fatalf("decision %d action = %s, want %s (%s)", i, got[i].Action, action, got[i].Reason)
		}
	}
}
