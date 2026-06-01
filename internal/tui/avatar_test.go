package tui

import "testing"

func TestFace_Expressions(t *testing.T) {
	cases := map[Expression]string{
		Resting: "(◕‿◕)",
		Blink:   "(•‿•)",
		Happy:   "(♥‿♥)",
		Sleepy:  "(˘‿˘)",
	}
	for e, want := range cases {
		if got := Face(e, 0); got != want {
			t.Fatalf("Face(%d,0) = %q, want %q", e, got, want)
		}
	}
}

func TestFace_WorkingAnimates(t *testing.T) {
	f0 := Face(Working, 0)
	f1 := Face(Working, 1)
	if f0 == f1 {
		t.Fatalf("Working face should change between frames: %q == %q", f0, f1)
	}
	// Frames wrap around.
	if Face(Working, 0) != Face(Working, 4) {
		t.Fatalf("Working frame 0 and 4 should match (cycle length 4)")
	}
}
