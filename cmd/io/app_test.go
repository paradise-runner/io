package main

import (
	"os"
	"testing"

	"github.com/edward-champion/io/internal/personastate"
)

func TestNewApp_MissingSoulDefersPersonaStart(t *testing.T) {
	root := t.TempDir()
	a, err := newApp(root, personastate.State{}, runtimeConfig{})
	if err != nil {
		t.Fatalf("newApp error: %v", err)
	}
	if !a.NeedsSetup() {
		t.Fatal("NeedsSetup = false, want true without SOUL.md")
	}
	if a.p != nil {
		t.Fatal("persona should not start before setup")
	}
}

func TestNeedsSetup_FalseWhenSoulExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(personastate.SoulPath(root), []byte("personality"), 0o600); err != nil {
		t.Fatalf("write soul: %v", err)
	}
	got, err := needsSetup(root)
	if err != nil {
		t.Fatalf("needsSetup error: %v", err)
	}
	if got {
		t.Fatal("needsSetup = true, want false with SOUL.md")
	}
}
