package chatlog

import (
	"path/filepath"
	"testing"
)

func TestAppendLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if err := Append(path, Entry{Role: "you", Text: "hi io"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := Append(path, Entry{Role: "io", Text: "hihi ♡"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != (Entry{Role: "you", Text: "hi io"}) || got[1] != (Entry{Role: "io", Text: "hihi ♡"}) {
		t.Fatalf("entries out of order or wrong: %+v", got)
	}
}

func TestLoad_MissingIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	_ = Append(path, Entry{Role: "you", Text: "x"})
	if err := Clear(path); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	got, _ := Load(path)
	if len(got) != 0 {
		t.Fatalf("after clear len = %d, want 0", len(got))
	}
	// Clearing a missing file is fine.
	if err := Clear(path); err != nil {
		t.Fatalf("Clear missing: %v", err)
	}
}
