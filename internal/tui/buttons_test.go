package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBar_HotkeyAction(t *testing.T) {
	var bar Bar
	cases := map[string]Action{
		"ctrl+s": ActionSettings,
		"ctrl+n": ActionNewChat,
		"ctrl+o": ActionContext,
		"ctrl+r": ActionMemory,
	}
	for key, want := range cases {
		got, ok := bar.HotkeyAction(key)
		if !ok || got != want {
			t.Fatalf("HotkeyAction(%q) = %v,%v want %v,true", key, got, ok, want)
		}
	}
	if _, ok := bar.HotkeyAction("ctrl+x"); ok {
		t.Fatal("HotkeyAction(ctrl+x) should be false")
	}
}

func TestBar_HitTest(t *testing.T) {
	var bar Bar

	// x=0 is inside the first button.
	if a, ok := bar.HitTest(0); !ok || a != ActionSettings {
		t.Fatalf("HitTest(0) = %v,%v want Settings,true", a, ok)
	}

	// Just past the first button (in the separator gap) → miss.
	w0 := lipgloss.Width(bar.renderButton(barButtons[0]))
	if _, ok := bar.HitTest(w0); ok {
		t.Fatalf("HitTest(%d) in separator gap should miss", w0)
	}

	// Start of the second button → New chat.
	if a, ok := bar.HitTest(w0 + barSeparator); !ok || a != ActionNewChat {
		t.Fatalf("HitTest(%d) = %v,%v want NewChat,true", w0+barSeparator, a, ok)
	}

	// Negative and far-right → miss.
	if _, ok := bar.HitTest(-1); ok {
		t.Fatal("HitTest(-1) should miss")
	}
	if _, ok := bar.HitTest(100000); ok {
		t.Fatal("HitTest(huge) should miss")
	}
}
