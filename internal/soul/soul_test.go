package soul

import (
	"strings"
	"testing"
)

func TestPresets_HaveFourAndStableIDs(t *testing.T) {
	got := Presets()
	if len(got) != 4 {
		t.Fatalf("len(Presets()) = %d, want 4", len(got))
	}
	wantIDs := map[string]bool{
		"staff_engineer": false,
		"chief_of_staff": false,
		"pair_partner":   false,
		"blank_slate":    false,
	}
	for _, p := range got {
		if _, ok := wantIDs[p.ID]; !ok {
			t.Fatalf("unexpected preset id %q", p.ID)
		}
		wantIDs[p.ID] = true
		if p.Name == "" || p.Description == "" {
			t.Fatalf("preset %q missing Name or Description", p.ID)
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing preset id %q", id)
		}
	}
}

func TestRender_IncludesPersonaAndKnobs(t *testing.T) {
	out := Render(Choice{
		PresetID:    "staff_engineer",
		Verbosity:   Terse,
		Proactivity: Reactive,
	})
	if !strings.Contains(out, "Staff Engineer") {
		t.Fatalf("rendered SOUL.md missing persona name:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "terse") {
		t.Fatalf("rendered SOUL.md missing verbosity guidance:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "you are io") {
		t.Fatalf("rendered SOUL.md missing identity line:\n%s", out)
	}
	if !strings.Contains(out, "brief, chat-sized messages") {
		t.Fatalf("rendered SOUL.md missing brevity block:\n%s", out)
	}
}

func TestRender_UnknownPresetFallsBackToBlank(t *testing.T) {
	out := Render(Choice{PresetID: "does_not_exist"})
	if !strings.Contains(strings.ToLower(out), "you are io") {
		t.Fatalf("fallback render must still establish identity:\n%s", out)
	}
}
