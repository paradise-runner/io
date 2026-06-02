package dreamer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteAtomic renders decisions into MEMORY.md and replaces the file atomically.
func WriteAtomic(path, existing string, decisions []CurationDecision) error {
	next := RenderMemory(existing, decisions)
	if next == existing {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".MEMORY.md-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(next); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// RenderMemory appends curated decisions to the existing Markdown memory file.
func RenderMemory(existing string, decisions []CurationDecision) string {
	var writes, merges, updates []Candidate
	for _, d := range decisions {
		switch d.Action {
		case ActionWrite:
			writes = append(writes, d.Candidate)
		case ActionMerge:
			merges = append(merges, d.Candidate)
		case ActionUpdate:
			updates = append(updates, d.Candidate)
		}
	}
	if len(writes)+len(merges)+len(updates) == 0 {
		return existing
	}

	var b strings.Builder
	if strings.TrimSpace(existing) == "" {
		b.WriteString("# io Memory\n")
	} else {
		b.WriteString(strings.TrimRight(existing, "\n"))
		b.WriteString("\n")
	}
	writeSection(&b, "Durable Notes", writes)
	writeSection(&b, "Evidence Updates", merges)
	writeSection(&b, "Updates", updates)
	return b.String()
}

func writeSection(b *strings.Builder, title string, candidates []Candidate) {
	if len(candidates) == 0 {
		return
	}
	b.WriteString("\n## ")
	b.WriteString(title)
	b.WriteString("\n")
	for _, c := range candidates {
		fmt.Fprintf(b, "- %s\n", strings.TrimSpace(c.Insight))
		if len(c.Evidence) > 0 {
			fmt.Fprintf(b, "  - Evidence: %s\n", strings.Join(c.Evidence, "; "))
		}
		fmt.Fprintf(b, "  - Confidence: %.2f\n", c.Confidence)
		if len(c.Tags) > 0 {
			fmt.Fprintf(b, "  - Tags: %s\n", strings.Join(c.Tags, ", "))
		}
	}
}
