package tui

import (
	"strings"
	"testing"
)

func TestSplitSegments_ProseOnly(t *testing.T) {
	segs := splitSegments("hello there\nhow are you?")
	if len(segs) != 1 || segs[0].Kind != Prose {
		t.Fatalf("got %+v, want one Prose segment", segs)
	}
	if segs[0].Text != "hello there\nhow are you?" {
		t.Fatalf("Text = %q", segs[0].Text)
	}
}

func TestSplitSegments_FencedCode(t *testing.T) {
	segs := splitSegments("```go\nx := 1\n```")
	if len(segs) != 1 || segs[0].Kind != Code {
		t.Fatalf("got %+v, want one Code segment", segs)
	}
	if segs[0].Lang != "go" {
		t.Fatalf("Lang = %q, want go", segs[0].Lang)
	}
	if segs[0].Text != "x := 1" {
		t.Fatalf("Text = %q, want 'x := 1'", segs[0].Text)
	}
}

func TestSplitSegments_Table(t *testing.T) {
	segs := splitSegments("| Key | Summary |\n|-----|---------|\n| ACE-1 | redesign |")
	if len(segs) != 1 || segs[0].Kind != Table {
		t.Fatalf("got %+v, want one Table segment", segs)
	}
}

func TestSplitSegments_Mixed(t *testing.T) {
	segs := splitSegments("here you go ♡\n```\ncode\n```\nthat's it!")
	if len(segs) != 3 {
		t.Fatalf("got %d segments, want 3: %+v", len(segs), segs)
	}
	if segs[0].Kind != Prose || segs[1].Kind != Code || segs[2].Kind != Prose {
		t.Fatalf("kinds = %v %v %v, want Prose Code Prose", segs[0].Kind, segs[1].Kind, segs[2].Kind)
	}
}

func TestRenderSegment_ProseIsBubbleWithText(t *testing.T) {
	out := renderSegment(Segment{Kind: Prose, Text: "hihi"}, "(◕‿◕)", 80)
	if !strings.Contains(out, "hihi") {
		t.Fatalf("prose render missing text:\n%s", out)
	}
	if !strings.Contains(out, "io") {
		t.Fatalf("prose render missing io label:\n%s", out)
	}
}

func TestRenderSegment_CodePanelHasTitle(t *testing.T) {
	out := renderSegment(Segment{Kind: Code, Text: "x := 1", Lang: "go"}, "(◕‿◕)", 80)
	if !strings.Contains(out, "go") {
		t.Fatalf("code panel missing lang title:\n%s", out)
	}
	if !strings.Contains(out, "x := 1") {
		t.Fatalf("code panel missing body:\n%s", out)
	}
}
