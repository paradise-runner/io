package claudeproc

import "testing"

func TestVersion(t *testing.T) {
	if Version != "phase1" {
		t.Fatalf("Version = %q, want %q", Version, "phase1")
	}
}
