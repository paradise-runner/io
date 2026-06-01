package tui

import (
	"strings"
	"testing"
)

func TestRenderBubble_YouIsRightAlignedWithLabel(t *testing.T) {
	out := renderBubble(RoleYou, "hi io", "", 80)
	if !strings.Contains(out, "hi io") {
		t.Fatalf("missing text:\n%s", out)
	}
	if !strings.Contains(out, "you") {
		t.Fatalf("missing you label:\n%s", out)
	}
	// Right-aligned: the first content line should begin with whitespace padding.
	first := strings.SplitN(out, "\n", 2)[0]
	if !strings.HasPrefix(first, " ") {
		t.Fatalf("you bubble should be right-aligned (leading space), got:\n%q", first)
	}
}

func TestRenderBubble_IOHasAvatarGutter(t *testing.T) {
	out := renderBubble(RoleIO, "hihi", "(◕‿◕)", 80)
	if !strings.Contains(out, "(◕‿◕)") {
		t.Fatalf("io bubble missing avatar face:\n%s", out)
	}
	if !strings.Contains(out, "io") {
		t.Fatalf("io bubble missing label:\n%s", out)
	}
}
