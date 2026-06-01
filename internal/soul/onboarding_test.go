package soul

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSoul_CreatesWhenMissing(t *testing.T) {
	root := t.TempDir()
	soulPath := filepath.Join(root, "SOUL.md")

	created, err := EnsureSoul(soulPath, func() (Choice, error) {
		return Choice{PresetID: "pair_partner", Verbosity: Balanced, Proactivity: Reactive}, nil
	})
	if err != nil {
		t.Fatalf("EnsureSoul error: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true on first run")
	}
	b, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("reading SOUL.md: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("SOUL.md is empty")
	}
}

func TestEnsureSoul_NoopWhenPresent(t *testing.T) {
	root := t.TempDir()
	soulPath := filepath.Join(root, "SOUL.md")
	if err := os.WriteFile(soulPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	created, err := EnsureSoul(soulPath, func() (Choice, error) {
		called = true
		return Choice{}, nil
	})
	if err != nil {
		t.Fatalf("EnsureSoul error: %v", err)
	}
	if created {
		t.Fatal("created = true, want false when SOUL.md already exists")
	}
	if called {
		t.Fatal("choice callback called, want skipped when SOUL.md exists")
	}
}
