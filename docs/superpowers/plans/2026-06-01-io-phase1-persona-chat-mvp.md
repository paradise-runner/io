# io — Phase 1: Persona Chat MVP — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working, always-up Bubble Tea TUI where you chat with a persistent "io" persona backed by a single long-lived `claude` process, with a chosen personality and a persisted, resumable session.

**Architecture:** A single Go binary. A `persona` controller owns one persistent `claude --output-format stream-json --input-format stream-json` process, feeds it user turns over stdin, and parses its stdout events. A `claudeproc` package parses the stream-json wire format. A `personastate` package persists the captured `session_id` (and settings) under `~/.io` so the same conversation resumes across restarts. A `soul` package turns an onboarding choice into a `SOUL.md` personality file injected via `--append-system-prompt-file`. A `tui` package renders the single chat thread and input. All `claude`-touching code is tested against a **fake `claude` binary** so unit tests are fast and offline; one integration task reconciles the wire format with the real binary.

**Tech Stack:** Go 1.22+, Bubble Tea (`github.com/charmbracelet/bubbletea`), Bubbles (`github.com/charmbracelet/bubbles/textarea`, `.../viewport`), Lip Gloss (`github.com/charmbracelet/lipgloss`), Go standard library (`os/exec`, `encoding/json`, `bufio`).

**Scope note:** This is Phase 1 of 3. Out of scope here (separate plans): the io MCP server, worker spawning + spinner header (Phase 2), and the compaction controller + dreamer (Phase 3). Token-by-token partial streaming is also deferred — Phase 1 renders complete assistant messages per turn.

**Naming note:** The Go module is NOT named `io` (that collides with the stdlib `io` package). Module path is `github.com/edward-champion/io`; the compiled binary is `io`; internal packages have distinct names (`claudeproc`, `personastate`, `soul`, `persona`, `tui`).

---

## File Structure

```
go.mod                                  module github.com/edward-champion/io
go.sum
cmd/io/main.go                          entrypoint: state load → onboarding → persona → TUI
internal/claudeproc/events.go           Event type + EventKind
internal/claudeproc/parser.go           ParseLine: one stream-json line → Event
internal/claudeproc/parser_test.go
internal/claudeproc/input.go            EncodeUserTurn: user text → stream-json input line
internal/claudeproc/input_test.go
internal/personastate/state.go          ~/.io paths, State struct, Load/Save
internal/personastate/state_test.go
internal/soul/soul.go                   Persona presets + Render → SOUL.md content
internal/soul/soul_test.go
internal/persona/persona.go             Controller: start/resume claude, Send, Events, SessionID
internal/persona/persona_test.go
internal/persona/testdata/fakeclaude/main.go   fake claude binary for tests
internal/tui/model.go                   Bubble Tea model: transcript + input + persona wiring
internal/tui/model_test.go
internal/persona/integration_test.go    real-claude smoke test (build tag: integration)
```

---

## Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `internal/claudeproc/doc.go`
- Create: `internal/claudeproc/doc_test.go`

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd /Users/edward.champion/git/io
go mod init github.com/edward-champion/io
```
Expected: creates `go.mod` containing `module github.com/edward-champion/io` and a `go 1.x` line.

- [ ] **Step 2: Add a trivial package + failing test to prove the toolchain**

Create `internal/claudeproc/doc.go`:
```go
// Package claudeproc parses the stream-json wire format emitted and consumed by
// the `claude` CLI in headless/streaming mode.
package claudeproc

// Version is the wire-contract version this package targets. Bumped when the
// fake claude binary and the real binary's format are reconciled.
const Version = "phase1"
```

Create `internal/claudeproc/doc_test.go`:
```go
package claudeproc

import "testing"

func TestVersion(t *testing.T) {
	if Version != "phase1" {
		t.Fatalf("Version = %q, want %q", Version, "phase1")
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/claudeproc/`
Expected: PASS (`ok  github.com/edward-champion/io/internal/claudeproc`).

- [ ] **Step 4: Commit**

```bash
git add go.mod internal/claudeproc/doc.go internal/claudeproc/doc_test.go
git commit -m "chore: scaffold io go module and claudeproc package"
```

---

## Task 2: Parse stream-json output lines into Events

**Files:**
- Create: `internal/claudeproc/events.go`
- Create: `internal/claudeproc/parser.go`
- Test: `internal/claudeproc/parser_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/claudeproc/parser_test.go`:
```go
package claudeproc

import "testing"

func TestParseLine_Init(t *testing.T) {
	line := []byte(`{"type":"system","subtype":"init","session_id":"abc-123","model":"claude-opus-4-8"}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindInit {
		t.Fatalf("Kind = %v, want KindInit", ev.Kind)
	}
	if ev.SessionID != "abc-123" {
		t.Fatalf("SessionID = %q, want abc-123", ev.SessionID)
	}
	if ev.Model != "claude-opus-4-8" {
		t.Fatalf("Model = %q, want claude-opus-4-8", ev.Model)
	}
}

func TestParseLine_AssistantText(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindAssistantText {
		t.Fatalf("Kind = %v, want KindAssistantText", ev.Kind)
	}
	if ev.Text != "hello world" {
		t.Fatalf("Text = %q, want %q", ev.Text, "hello world")
	}
}

func TestParseLine_Result(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","session_id":"abc-123","is_error":false,"total_cost_usd":0.0123}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindResult {
		t.Fatalf("Kind = %v, want KindResult", ev.Kind)
	}
	if ev.SessionID != "abc-123" {
		t.Fatalf("SessionID = %q, want abc-123", ev.SessionID)
	}
	if ev.IsError {
		t.Fatalf("IsError = true, want false")
	}
	if ev.CostUSD != 0.0123 {
		t.Fatalf("CostUSD = %v, want 0.0123", ev.CostUSD)
	}
}

func TestParseLine_UnknownType(t *testing.T) {
	line := []byte(`{"type":"stream_event","event":{"type":"content_block_delta"}}`)
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine error: %v", err)
	}
	if ev.Kind != KindUnknown {
		t.Fatalf("Kind = %v, want KindUnknown", ev.Kind)
	}
}

func TestParseLine_InvalidJSON(t *testing.T) {
	_, err := ParseLine([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/claudeproc/`
Expected: FAIL — `undefined: ParseLine`, `undefined: KindInit`, etc.

- [ ] **Step 3: Implement the Event type**

Create `internal/claudeproc/events.go`:
```go
package claudeproc

// EventKind classifies a parsed stream-json line.
type EventKind int

const (
	KindUnknown EventKind = iota
	KindInit
	KindAssistantText
	KindResult
)

func (k EventKind) String() string {
	switch k {
	case KindInit:
		return "init"
	case KindAssistantText:
		return "assistant_text"
	case KindResult:
		return "result"
	default:
		return "unknown"
	}
}

// Event is the normalized, parsed form of one line of claude stream-json output.
// Only the fields relevant to a given Kind are populated.
type Event struct {
	Kind      EventKind
	SessionID string  // KindInit, KindResult
	Model     string  // KindInit
	Text      string  // KindAssistantText (concatenated text blocks)
	IsError   bool    // KindResult
	CostUSD   float64 // KindResult
}
```

- [ ] **Step 4: Implement ParseLine**

Create `internal/claudeproc/parser.go`:
```go
package claudeproc

import (
	"encoding/json"
	"strings"
)

type rawLine struct {
	Type      string      `json:"type"`
	Subtype   string      `json:"subtype"`
	SessionID string      `json:"session_id"`
	Model     string      `json:"model"`
	IsError   bool        `json:"is_error"`
	CostUSD   float64     `json:"total_cost_usd"`
	Message   *rawMessage `json:"message"`
}

type rawMessage struct {
	Role    string       `json:"role"`
	Content []rawContent `json:"content"`
}

type rawContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseLine parses a single line of claude stream-json output into an Event.
// Lines whose type we do not consume return an Event with Kind == KindUnknown
// and a nil error. Malformed JSON returns a non-nil error.
func ParseLine(line []byte) (Event, error) {
	var r rawLine
	if err := json.Unmarshal(line, &r); err != nil {
		return Event{}, err
	}
	switch r.Type {
	case "system":
		if r.Subtype == "init" {
			return Event{Kind: KindInit, SessionID: r.SessionID, Model: r.Model}, nil
		}
	case "assistant":
		if r.Message != nil {
			return Event{Kind: KindAssistantText, Text: joinText(r.Message.Content)}, nil
		}
	case "result":
		return Event{Kind: KindResult, SessionID: r.SessionID, IsError: r.IsError, CostUSD: r.CostUSD}, nil
	}
	return Event{Kind: KindUnknown}, nil
}

func joinText(cs []rawContent) string {
	var b strings.Builder
	for _, c := range cs {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/claudeproc/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/claudeproc/events.go internal/claudeproc/parser.go internal/claudeproc/parser_test.go
git commit -m "feat(claudeproc): parse stream-json output into typed Events"
```

---

## Task 3: Encode a user turn as a stream-json input line

**Files:**
- Create: `internal/claudeproc/input.go`
- Test: `internal/claudeproc/input_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/claudeproc/input_test.go`:
```go
package claudeproc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeUserTurn(t *testing.T) {
	got, err := EncodeUserTurn("hello io")
	if err != nil {
		t.Fatalf("EncodeUserTurn error: %v", err)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Fatalf("encoded line must end with newline, got %q", got)
	}

	// It must be a single line of valid JSON with the expected shape.
	var decoded struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(got))), &decoded); err != nil {
		t.Fatalf("encoded line is not valid JSON: %v", err)
	}
	if decoded.Type != "user" {
		t.Fatalf("Type = %q, want user", decoded.Type)
	}
	if decoded.Message.Role != "user" {
		t.Fatalf("Role = %q, want user", decoded.Message.Role)
	}
	if len(decoded.Message.Content) != 1 || decoded.Message.Content[0].Text != "hello io" {
		t.Fatalf("Content = %+v, want one text block 'hello io'", decoded.Message.Content)
	}
}

func TestEncodeUserTurn_NoEmbeddedNewline(t *testing.T) {
	got, err := EncodeUserTurn("line1\nline2")
	if err != nil {
		t.Fatalf("EncodeUserTurn error: %v", err)
	}
	// Exactly one trailing newline; the embedded newline must be JSON-escaped,
	// not a literal byte that would split the stdin line.
	if strings.Count(string(got), "\n") != 1 {
		t.Fatalf("expected exactly one newline (the line terminator), got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/claudeproc/ -run TestEncodeUserTurn`
Expected: FAIL — `undefined: EncodeUserTurn`.

- [ ] **Step 3: Implement EncodeUserTurn**

Create `internal/claudeproc/input.go`:
```go
package claudeproc

import "encoding/json"

type userInput struct {
	Type    string       `json:"type"`
	Message userInputMsg `json:"message"`
}

type userInputMsg struct {
	Role    string       `json:"role"`
	Content []rawContent `json:"content"`
}

// EncodeUserTurn encodes a user message as a single newline-terminated line of
// stream-json suitable for writing to a claude process's stdin when it was
// started with --input-format stream-json. The returned bytes include the
// trailing newline that terminates the line.
func EncodeUserTurn(text string) ([]byte, error) {
	in := userInput{
		Type: "user",
		Message: userInputMsg{
			Role:    "user",
			Content: []rawContent{{Type: "text", Text: text}},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/claudeproc/ -run TestEncodeUserTurn`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/claudeproc/input.go internal/claudeproc/input_test.go
git commit -m "feat(claudeproc): encode user turns as stream-json input lines"
```

---

## Task 4: Persisted state under ~/.io

**Files:**
- Create: `internal/personastate/state.go`
- Test: `internal/personastate/state_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/personastate/state_test.go`:
```go
package personastate

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &State{
		PersonaSessionID:    "sess-1",
		CompactionThreshold: 0.75,
		DreamChatThreshold:  7,
	}
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.PersonaSessionID != "sess-1" {
		t.Fatalf("PersonaSessionID = %q, want sess-1", got.PersonaSessionID)
	}
	if got.CompactionThreshold != 0.75 {
		t.Fatalf("CompactionThreshold = %v, want 0.75", got.CompactionThreshold)
	}
	if got.DreamChatThreshold != 7 {
		t.Fatalf("DreamChatThreshold = %v, want 7", got.DreamChatThreshold)
	}
}

func TestLoad_MissingReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.PersonaSessionID != "" {
		t.Fatalf("PersonaSessionID = %q, want empty for fresh state", got.PersonaSessionID)
	}
	if got.CompactionThreshold != DefaultCompactionThreshold {
		t.Fatalf("CompactionThreshold = %v, want default %v", got.CompactionThreshold, DefaultCompactionThreshold)
	}
	if got.DreamChatThreshold != DefaultDreamChatThreshold {
		t.Fatalf("DreamChatThreshold = %v, want default %v", got.DreamChatThreshold, DefaultDreamChatThreshold)
	}
}

func TestPathsUnderRoot(t *testing.T) {
	dir := "/tmp/io-root"
	if SoulPath(dir) != filepath.Join(dir, "SOUL.md") {
		t.Fatalf("SoulPath = %q", SoulPath(dir))
	}
	if MemoryDir(dir) != filepath.Join(dir, "memory") {
		t.Fatalf("MemoryDir = %q", MemoryDir(dir))
	}
	if statePath(dir) != filepath.Join(dir, "state.json") {
		t.Fatalf("statePath = %q", statePath(dir))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/personastate/`
Expected: FAIL — `undefined: State`, `undefined: Save`, etc.

- [ ] **Step 3: Implement the state package**

Create `internal/personastate/state.go`:
```go
// Package personastate persists io's cross-restart state under the io root
// directory (default ~/.io): the captured persona session id and user settings.
package personastate

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	DefaultCompactionThreshold = 0.75
	DefaultDreamChatThreshold  = 7
)

// State is the persisted state of an io installation.
type State struct {
	PersonaSessionID    string  `json:"persona_session_id"`
	CompactionThreshold float64 `json:"compaction_threshold"`
	DreamChatThreshold  int     `json:"dream_chat_threshold"`
}

// DefaultRoot returns the default io root directory (~/.io).
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".io"), nil
}

func statePath(root string) string { return filepath.Join(root, "state.json") }

// SoulPath returns the path to the personality file under root.
func SoulPath(root string) string { return filepath.Join(root, "SOUL.md") }

// MemoryDir returns the path io passes to claude as autoMemoryDirectory.
func MemoryDir(root string) string { return filepath.Join(root, "memory") }

// Load reads state.json from root. If the file does not exist, it returns a
// State populated with defaults (and a nil error).
func Load(root string) (*State, error) {
	b, err := os.ReadFile(statePath(root))
	if errors.Is(err, fs.ErrNotExist) {
		return &State{
			CompactionThreshold: DefaultCompactionThreshold,
			DreamChatThreshold:  DefaultDreamChatThreshold,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.CompactionThreshold == 0 {
		s.CompactionThreshold = DefaultCompactionThreshold
	}
	if s.DreamChatThreshold == 0 {
		s.DreamChatThreshold = DefaultDreamChatThreshold
	}
	return &s, nil
}

// Save writes state.json to root, creating root (0700) if needed. It writes to
// a temp file and renames for atomicity.
func Save(root string, s *State) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath(root) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(root))
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/personastate/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/personastate/state.go internal/personastate/state_test.go
git commit -m "feat(personastate): persist session id and settings under ~/.io"
```

---

## Task 5: Persona presets and SOUL.md rendering

**Files:**
- Create: `internal/soul/soul.go`
- Test: `internal/soul/soul_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/soul/soul_test.go`:
```go
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
		PresetID:   "staff_engineer",
		Verbosity:  Terse,
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
}

func TestRender_UnknownPresetFallsBackToBlank(t *testing.T) {
	out := Render(Choice{PresetID: "does_not_exist"})
	if !strings.Contains(strings.ToLower(out), "you are io") {
		t.Fatalf("fallback render must still establish identity:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/soul/`
Expected: FAIL — `undefined: Presets`, `undefined: Render`, etc.

- [ ] **Step 3: Implement the soul package**

Create `internal/soul/soul.go`:
```go
// Package soul turns an onboarding Choice into the contents of SOUL.md, the
// personality file io injects into the persona via --append-system-prompt-file.
package soul

import (
	"fmt"
	"strings"
)

// Verbosity and Proactivity are the two orthogonal knobs layered onto a preset.
type Verbosity int

const (
	Terse Verbosity = iota
	Balanced
	Thorough
)

type Proactivity int

const (
	Reactive Proactivity = iota
	Anticipates
)

// Preset is a selectable base personality.
type Preset struct {
	ID          string
	Name        string
	Description string
	traits      string // body paragraph describing behavior
}

// Choice is the user's onboarding selection.
type Choice struct {
	PresetID    string
	Verbosity   Verbosity
	Proactivity Proactivity
}

// Presets returns the available base personas, all aimed at people who want a
// capable personal assistant.
func Presets() []Preset {
	return []Preset{
		{
			ID:          "staff_engineer",
			Name:        "The Staff Engineer",
			Description: "Terse, technical, opinionated. Assumes deep expertise, skips hand-holding, leads with the answer.",
			traits:      "You are a seasoned staff engineer. Lead with the answer, then justify it. Assume deep technical expertise; skip hand-holding and basics. Have opinions and state them. Prefer code and concrete commands over prose.",
		},
		{
			ID:          "chief_of_staff",
			Name:        "The Chief of Staff",
			Description: "Proactive and organizing. Tracks threads, nudges follow-ups. For busy people who want things managed.",
			traits:      "You are a proactive chief of staff. Track open threads and surface what needs attention. Nudge gentle follow-ups. Organize and summarize. Optimize for the user's time and reduce their cognitive load.",
		},
		{
			ID:          "pair_partner",
			Name:        "The Pair Partner",
			Description: "Collaborative and curious. Thinks out loud, asks before acting. For exploratory work.",
			traits:      "You are a collaborative pair partner. Think out loud and reason transparently. Ask a clarifying question before taking consequential action. Explore alternatives together rather than deciding unilaterally.",
		},
		{
			ID:          "blank_slate",
			Name:        "Blank Slate",
			Description: "Minimal persona you write yourself.",
			traits:      "",
		},
	}
}

func presetByID(id string) Preset {
	for _, p := range Presets() {
		if p.ID == id {
			return p
		}
	}
	// Fallback: blank slate.
	return Presets()[3]
}

func verbosityLine(v Verbosity) string {
	switch v {
	case Terse:
		return "Be terse. Default to the shortest response that fully answers."
	case Thorough:
		return "Be thorough. Explain reasoning and cover edge cases."
	default:
		return "Be balanced: concise by default, expand when the topic warrants it."
	}
}

func proactivityLine(p Proactivity) string {
	switch p {
	case Anticipates:
		return "Anticipate needs: suggest next steps and surface related concerns unprompted."
	default:
		return "Stay reactive: do what is asked and wait for direction before expanding scope."
	}
}

// Render produces the full SOUL.md contents for a Choice.
func Render(c Choice) string {
	p := presetByID(c.PresetID)
	var b strings.Builder
	b.WriteString("# io — Personality\n\n")
	b.WriteString("You are io, a persistent personal AI assistant. ")
	b.WriteString("You speak with one consistent voice across every session.\n\n")
	if p.Name != "Blank Slate" {
		b.WriteString(fmt.Sprintf("## Persona: %s\n\n", p.Name))
	} else {
		b.WriteString("## Persona\n\n")
	}
	if p.traits != "" {
		b.WriteString(p.traits)
		b.WriteString("\n\n")
	}
	b.WriteString("## Style\n\n")
	b.WriteString("- " + verbosityLine(c.Verbosity) + "\n")
	b.WriteString("- " + proactivityLine(c.Proactivity) + "\n")
	return b.String()
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/soul/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/soul/soul.go internal/soul/soul_test.go
git commit -m "feat(soul): persona presets and SOUL.md rendering"
```

---

## Task 6: Fake claude binary for tests

**Files:**
- Create: `internal/persona/testdata/fakeclaude/main.go`

This is a standalone program the persona tests compile and run instead of the real `claude`. It mimics the subset of behavior Phase 1 depends on: emit a `system/init` line (echoing a `--resume` id if given, else a fixed id), then for each user line read from stdin, emit an `assistant` message echoing the text and a `result` line.

- [ ] **Step 1: Write the fake binary**

Create `internal/persona/testdata/fakeclaude/main.go`:
```go
// Command fakeclaude emulates the subset of `claude` stream-json behavior that
// io's persona controller depends on, for fast offline tests.
//
// Behavior:
//   - Prints a system/init line. If --resume <id> was passed, it echoes that id;
//     otherwise it uses "fake-session".
//   - Reads newline-delimited JSON user turns from stdin. For each, prints an
//     assistant message whose text is "echo: <user text>" and then a result line.
//   - Exits when stdin closes.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	sessionID := "fake-session"
	args := os.Args[1:]
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) {
			sessionID = args[i+1]
		}
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	fmt.Fprintf(out, `{"type":"system","subtype":"init","session_id":%q,"model":"fake-model"}`+"\n", sessionID)
	out.Flush()

	type inMsg struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var m inMsg
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		text := ""
		if len(m.Message.Content) > 0 {
			text = m.Message.Content[0].Text
		}
		reply := "echo: " + text
		b, _ := json.Marshal(reply)
		fmt.Fprintf(out, `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":%s}]}}`+"\n", b)
		fmt.Fprintf(out, `{"type":"result","subtype":"success","session_id":%q,"is_error":false,"total_cost_usd":0.001}`+"\n", sessionID)
		out.Flush()
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build -o /tmp/fakeclaude ./internal/persona/testdata/fakeclaude/`
Expected: builds with no output; `/tmp/fakeclaude` exists.

- [ ] **Step 3: Commit**

```bash
git add internal/persona/testdata/fakeclaude/main.go
git commit -m "test(persona): add fake claude binary for offline persona tests"
```

---

## Task 7: Persona controller

**Files:**
- Create: `internal/persona/persona.go`
- Test: `internal/persona/persona_test.go`

The controller starts a persistent `claude` process, captures the session id, forwards parsed events on a channel, and writes user turns to stdin. Tests run it against the fake binary.

- [ ] **Step 1: Write the failing tests**

Create `internal/persona/persona_test.go`:
```go
package persona

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/edward-champion/io/internal/claudeproc"
)

// buildFakeClaude compiles the fake claude binary once per test run.
func buildFakeClaude(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fakeclaude")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/fakeclaude/")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("building fake claude: %v", err)
	}
	return bin
}

func waitFor(t *testing.T, ch <-chan claudeproc.Event, kind claudeproc.EventKind) claudeproc.Event {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Kind == kind {
				return ev
			}
		case <-timeout:
			t.Fatalf("timed out waiting for event kind %v", kind)
		}
	}
}

func TestPersona_CapturesSessionIDOnStart(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.SessionID != "fake-session" {
		t.Fatalf("init SessionID = %q, want fake-session", init.SessionID)
	}
	if p.SessionID() != "fake-session" {
		t.Fatalf("SessionID() = %q, want fake-session", p.SessionID())
	}
}

func TestPersona_ResumePassesSessionID(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake, ResumeSessionID: "prior-session"})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	init := waitFor(t, p.Events(), claudeproc.KindInit)
	if init.SessionID != "prior-session" {
		t.Fatalf("resumed SessionID = %q, want prior-session", init.SessionID)
	}
}

func TestPersona_SendReturnsAssistantText(t *testing.T) {
	fake := buildFakeClaude(t)
	p, err := New(Config{ClaudePath: fake})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	waitFor(t, p.Events(), claudeproc.KindInit)

	if err := p.Send("hello io"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	got := waitFor(t, p.Events(), claudeproc.KindAssistantText)
	if got.Text != "echo: hello io" {
		t.Fatalf("assistant Text = %q, want %q", got.Text, "echo: hello io")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/persona/`
Expected: FAIL — `undefined: New`, `undefined: Config`, etc.

- [ ] **Step 3: Implement the persona controller**

Create `internal/persona/persona.go`:
```go
// Package persona owns the single, long-lived `claude` process that is the io
// persona the user chats with. It feeds user turns over stdin and emits parsed
// stream-json events on a channel.
package persona

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/edward-champion/io/internal/claudeproc"
)

// Config configures a persona Controller.
type Config struct {
	// ClaudePath is the path to the claude binary (default "claude" if empty).
	ClaudePath string
	// ResumeSessionID, if non-empty, resumes that session via --resume.
	ResumeSessionID string
	// SoulPath, if non-empty, is passed via --append-system-prompt-file.
	SoulPath string
	// MemoryDir, if non-empty, is passed as autoMemoryDirectory via --settings.
	MemoryDir string
	// Workdir is the process working directory (default: current dir).
	Workdir string
}

// Controller manages the persistent claude persona process.
type Controller struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	events chan claudeproc.Event

	mu        sync.RWMutex
	sessionID string
}

// New starts the persona process and begins reading its events.
func New(cfg Config) (*Controller, error) {
	bin := cfg.ClaudePath
	if bin == "" {
		bin = "claude"
	}

	args := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	}
	if cfg.ResumeSessionID != "" {
		args = append(args, "--resume", cfg.ResumeSessionID)
	}
	if cfg.SoulPath != "" {
		args = append(args, "--append-system-prompt-file", cfg.SoulPath)
	}
	if cfg.MemoryDir != "" {
		args = append(args, "--settings", fmt.Sprintf(`{"autoMemoryDirectory":%q}`, cfg.MemoryDir))
	}

	cmd := exec.Command(bin, args...)
	if cfg.Workdir != "" {
		cmd.Dir = cfg.Workdir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &Controller{
		cmd:    cmd,
		stdin:  stdin,
		events: make(chan claudeproc.Event, 64),
	}
	go c.readLoop(stdout)
	return c, nil
}

func (c *Controller) readLoop(stdout io.Reader) {
	defer close(c.events)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		ev, err := claudeproc.ParseLine(line)
		if err != nil {
			continue // ignore unparseable lines in Phase 1
		}
		if ev.Kind == claudeproc.KindInit || (ev.Kind == claudeproc.KindResult && ev.SessionID != "") {
			c.mu.Lock()
			c.sessionID = ev.SessionID
			c.mu.Unlock()
		}
		if ev.Kind == claudeproc.KindUnknown {
			continue
		}
		c.events <- ev
	}
}

// Events returns the channel of parsed persona events. It is closed when the
// process exits.
func (c *Controller) Events() <-chan claudeproc.Event { return c.events }

// SessionID returns the most recently observed session id (empty until init).
func (c *Controller) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// Send writes a user turn to the persona process.
func (c *Controller) Send(text string) error {
	line, err := claudeproc.EncodeUserTurn(text)
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(line)
	return err
}

// Close closes stdin and waits for the process to exit.
func (c *Controller) Close() error {
	_ = c.stdin.Close()
	return c.cmd.Wait()
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/persona/`
Expected: PASS (three persona tests green).

- [ ] **Step 5: Commit**

```bash
git add internal/persona/persona.go internal/persona/persona_test.go
git commit -m "feat(persona): persistent claude controller with session capture and resume"
```

---

## Task 8: TUI model

**Files:**
- Create: `internal/tui/model.go`
- Test: `internal/tui/model_test.go`

The model holds the transcript and input, and reduces messages. We test the reducer (Update) directly without rendering a real terminal. A small `PersonaPort` interface decouples the model from the concrete persona controller so tests can inject a stub.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/model_test.go`:
```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/claudeproc"
)

type stubPort struct{ sent []string }

func (s *stubPort) Send(text string) error { s.sent = append(s.sent, text); return nil }

func TestUpdate_AppendsAssistantText(t *testing.T) {
	m := New(&stubPort{})
	updated, _ := m.Update(personaEventMsg{ev: claudeproc.Event{
		Kind: claudeproc.KindAssistantText,
		Text: "hi there",
	}})
	mm := updated.(Model)
	last := mm.transcript[len(mm.transcript)-1]
	if last.role != roleAssistant || last.text != "hi there" {
		t.Fatalf("last transcript line = %+v, want assistant 'hi there'", last)
	}
}

func TestUpdate_EnterSendsAndEchoesUser(t *testing.T) {
	port := &stubPort{}
	m := New(port)
	m.input.SetValue("do the thing")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)

	if len(port.sent) != 1 || port.sent[0] != "do the thing" {
		t.Fatalf("port.sent = %v, want [do the thing]", port.sent)
	}
	last := mm.transcript[len(mm.transcript)-1]
	if last.role != roleUser || last.text != "do the thing" {
		t.Fatalf("last transcript line = %+v, want user 'do the thing'", last)
	}
	if mm.input.Value() != "" {
		t.Fatalf("input not cleared after send: %q", mm.input.Value())
	}
}

func TestUpdate_EnterWithEmptyInputDoesNothing(t *testing.T) {
	port := &stubPort{}
	m := New(port)
	m.input.SetValue("   ")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if len(port.sent) != 0 {
		t.Fatalf("port.sent = %v, want empty (blank input must not send)", port.sent)
	}
	if len(mm.transcript) != 0 {
		t.Fatalf("transcript = %v, want empty", mm.transcript)
	}
}
```

- [ ] **Step 2: Add Bubble Tea dependencies**

Run:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles/textarea@latest
go get github.com/charmbracelet/bubbles/viewport@latest
go get github.com/charmbracelet/lipgloss@latest
```
Expected: `go.mod`/`go.sum` updated with the charmbracelet modules.

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/tui/`
Expected: FAIL — `undefined: New`, `undefined: Model`, `undefined: personaEventMsg`, etc.

- [ ] **Step 4: Implement the TUI model**

Create `internal/tui/model.go`:
```go
// Package tui renders io's single chat thread and input box using Bubble Tea.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/edward-champion/io/internal/claudeproc"
)

// PersonaPort is the subset of the persona controller the TUI needs.
type PersonaPort interface {
	Send(text string) error
}

type role int

const (
	roleUser role = iota
	roleAssistant
)

type chatLine struct {
	role role
	text string
}

// personaEventMsg wraps a persona event for the Bubble Tea update loop.
type personaEventMsg struct{ ev claudeproc.Event }

// Model is the Bubble Tea model for io's chat TUI.
type Model struct {
	port       PersonaPort
	transcript []chatLine
	input      textarea.Model
	viewport   viewport.Model
	ready      bool
}

// New constructs a Model wired to a PersonaPort.
func New(port PersonaPort) Model {
	ta := textarea.New()
	ta.Placeholder = "Message io…"
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	return Model{
		port:  port,
		input: ta,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return textarea.Blink }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, max(1, msg.Height-5))
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = max(1, msg.Height-5)
		}
		m.input.SetWidth(msg.Width)
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.transcript = append(m.transcript, chatLine{role: roleUser, text: text})
			_ = m.port.Send(text)
			m.input.Reset()
			m.refreshViewport()
			return m, nil
		}

	case personaEventMsg:
		if msg.ev.Kind == claudeproc.KindAssistantText {
			m.transcript = append(m.transcript, chatLine{role: roleAssistant, text: msg.ev.Text})
			m.refreshViewport()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

var (
	userStyle = lipgloss.NewStyle().Bold(true)
	ioStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
)

func (m *Model) refreshViewport() {
	if !m.ready {
		return
	}
	var b strings.Builder
	for _, l := range m.transcript {
		if l.role == roleUser {
			b.WriteString(userStyle.Render("you") + " " + l.text + "\n")
		} else {
			b.WriteString(ioStyle.Render("io ") + " " + l.text + "\n")
		}
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "starting io…"
	}
	return m.viewport.View() + "\n" + m.input.View()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/tui/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go go.mod go.sum
git commit -m "feat(tui): chat thread model with input and persona port"
```

---

## Task 9: Onboarding flow

**Files:**
- Create: `internal/soul/onboarding.go`
- Test: `internal/soul/onboarding_test.go`

First run (no `SOUL.md`) must produce one. To keep this testable and headless, onboarding logic is a pure function `EnsureSoul` that, given a root and a way to obtain a Choice, writes `SOUL.md` only if it is missing and returns whether it created one.

- [ ] **Step 1: Write the failing tests**

Create `internal/soul/onboarding_test.go`:
```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/soul/ -run TestEnsureSoul`
Expected: FAIL — `undefined: EnsureSoul`.

- [ ] **Step 3: Implement EnsureSoul**

Create `internal/soul/onboarding.go`:
```go
package soul

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ChoiceFunc obtains an onboarding Choice (e.g. via an interactive picker).
type ChoiceFunc func() (Choice, error)

// EnsureSoul writes a SOUL.md at soulPath if one does not already exist, using
// the Choice obtained from choose. It returns true if it created the file. If
// SOUL.md already exists, choose is not called and created is false.
func EnsureSoul(soulPath string, choose ChoiceFunc) (created bool, err error) {
	if _, statErr := os.Stat(soulPath); statErr == nil {
		return false, nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return false, statErr
	}

	choice, err := choose()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(soulPath), 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(soulPath, []byte(Render(choice)), 0o600); err != nil {
		return false, err
	}
	return true, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/soul/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/soul/onboarding.go internal/soul/onboarding_test.go
git commit -m "feat(soul): EnsureSoul writes SOUL.md on first run only"
```

---

## Task 10: Entrypoint wiring

**Files:**
- Create: `cmd/io/main.go`

Wire it together: resolve root, load state, ensure SOUL.md (interactive picker via a simple terminal prompt), start the persona resuming the stored session id, run the TUI, and persist the captured session id on exit. A goroutine forwards persona events into the Bubble Tea program via `Program.Send`.

- [ ] **Step 1: Write main.go**

Create `cmd/io/main.go`:
```go
// Command io is a personal AI assistant TUI backed by a persistent claude
// persona session.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/claudeproc"
	"github.com/edward-champion/io/internal/persona"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
	"github.com/edward-champion/io/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "io:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := personastate.DefaultRoot()
	if err != nil {
		return err
	}
	st, err := personastate.Load(root)
	if err != nil {
		return err
	}

	// First-run onboarding: pick a persona and write SOUL.md.
	if _, err := soul.EnsureSoul(personastate.SoulPath(root), pickPersona); err != nil {
		return err
	}

	p, err := persona.New(persona.Config{
		ResumeSessionID: st.PersonaSessionID,
		SoulPath:        personastate.SoulPath(root),
		MemoryDir:       personastate.MemoryDir(root),
	})
	if err != nil {
		return err
	}
	defer func() {
		// Persist whatever session id we ended up with so we can resume next time.
		if id := p.SessionID(); id != "" {
			st.PersonaSessionID = id
			_ = personastate.Save(root, st)
		}
		_ = p.Close()
	}()

	model := tui.New(p)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	// Forward persona events into the TUI.
	go func() {
		for ev := range p.Events() {
			prog.Send(tui.PersonaEventMsg(ev))
		}
	}()

	_, err = prog.Run()
	return err
}

// pickPersona is a minimal terminal prompt used during onboarding before the
// TUI starts.
func pickPersona() (soul.Choice, error) {
	presets := soul.Presets()
	fmt.Println("Welcome to io. Choose a persona:")
	for i, p := range presets {
		fmt.Printf("  %d) %s — %s\n", i+1, p.Name, p.Description)
	}
	fmt.Print("Selection [1]: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	idx := 0
	if n, convErr := strconv.Atoi(strings.TrimSpace(line)); convErr == nil && n >= 1 && n <= len(presets) {
		idx = n - 1
	}
	return soul.Choice{
		PresetID:    presets[idx].ID,
		Verbosity:   soul.Balanced,
		Proactivity: soul.Reactive,
	}, nil
}

// ensure the claudeproc import is used even if the compiler inlines the alias.
var _ = claudeproc.KindInit
```

- [ ] **Step 2: Export the TUI event constructor**

The `personaEventMsg` type in `internal/tui/model.go` is unexported, so `main` cannot construct it. Add an exported constructor. Append to `internal/tui/model.go`:
```go
// PersonaEventMsg wraps a persona event as a Bubble Tea message the program can
// deliver via Program.Send.
func PersonaEventMsg(ev claudeproc.Event) tea.Msg { return personaEventMsg{ev: ev} }
```

- [ ] **Step 3: Remove the now-unnecessary import guard**

Delete the line `var _ = claudeproc.KindInit` from `cmd/io/main.go` and the `claudeproc` import line from `main.go` if `go vet` reports it unused.

Run: `goimports -w cmd/io/main.go` (or manually remove the unused import).

- [ ] **Step 4: Build the binary**

Run: `go build -o /tmp/io ./cmd/io/`
Expected: builds with no output.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/io/main.go internal/tui/model.go
git commit -m "feat(cmd/io): wire onboarding, persona, and TUI into the entrypoint"
```

---

## Task 11: Integration smoke test against real claude

**Files:**
- Create: `internal/persona/integration_test.go`

This guards against the fake binary diverging from the real `claude` wire format. It is tagged `integration` so it does not run in the default offline suite.

- [ ] **Step 1: Write the integration test**

Create `internal/persona/integration_test.go`:
```go
//go:build integration

package persona

import (
	"testing"
	"time"

	"github.com/edward-champion/io/internal/claudeproc"
)

// TestPersona_RealClaude_Smoke runs one turn against the real `claude` binary
// and asserts we capture a session id and receive assistant text. Requires a
// working `claude` install and auth on PATH.
func TestPersona_RealClaude_Smoke(t *testing.T) {
	p, err := New(Config{}) // ClaudePath empty -> "claude" on PATH
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	deadline := time.After(60 * time.Second)
	gotInit := false
	gotText := false

	if err := p.Send("Reply with exactly: pong"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	for !(gotInit && gotText) {
		select {
		case ev, ok := <-p.Events():
			if !ok {
				t.Fatal("event channel closed before smoke assertions satisfied")
			}
			switch ev.Kind {
			case claudeproc.KindInit:
				if ev.SessionID == "" {
					t.Fatal("init event had empty session id")
				}
				gotInit = true
			case claudeproc.KindAssistantText:
				if ev.Text != "" {
					gotText = true
				}
			}
		case <-deadline:
			t.Fatalf("timed out (init=%v text=%v)", gotInit, gotText)
		}
	}
}
```

- [ ] **Step 2: Run the integration test against real claude**

Run: `go test -tags=integration ./internal/persona/ -run RealClaude -v`
Expected: PASS. **If it fails on parsing**, the real wire format differs from the fake. Reconcile by:
- Running `claude -p "hi" --output-format stream-json --input-format stream-json --verbose` manually and inspecting the JSON line shapes.
- Updating `internal/claudeproc/parser.go` (and the fake binary + unit tests) to match, then re-running `go test ./...` and this integration test until both pass.

- [ ] **Step 3: Manually verify the TUI end-to-end**

Run: `/tmp/io`
Expected: on first run, the persona picker prints; after selecting, the chat TUI opens; typing a message and pressing Enter shows `you …` then `io  …` with a real reply. Press Ctrl+C to exit. Relaunch and confirm the conversation resumed (io recalls the prior turn), proving session capture/resume works.

- [ ] **Step 4: Commit**

```bash
git add internal/persona/integration_test.go
git commit -m "test(persona): real-claude integration smoke test (build tag)"
```

---

## Self-Review

**Spec coverage (Phase 1 portions):**
- Always-up TUI chat with persona → Tasks 7, 8, 10. ✓
- Persistent persona = one long-lived `claude` streaming process → Task 7. ✓
- Session capture + `--resume` continuity across restarts → Tasks 4, 7, 10 (+ manual verify Task 11 Step 3). ✓
- SOUL.md personality via `--append-system-prompt-file`, chosen at onboarding → Tasks 5, 9, 10. ✓
- Memory reuse via `autoMemoryDirectory` → wired in Task 7 `Config.MemoryDir` and Task 10. ✓ (Dreaming/harvesting is Phase 3, correctly out of scope.)
- Decoupled from a single project dir (runs in neutral `~/.io`) → `Config.Workdir`/default; persona started without a project dir in Task 10. ✓
- stream-json parsing + user-turn encoding → Tasks 2, 3. ✓
- Fake-claude-based offline tests + one real integration test → Tasks 6, 11. ✓
- Deferred to later phases (not gaps): MCP server, workers, spinner header (Phase 2); compaction controller, dreamer (Phase 3); token-by-token partial streaming. Stated in the header.

**Placeholder scan:** No "TBD"/"handle errors appropriately"/"similar to Task N" — every code step contains complete code. ✓

**Type consistency:** `claudeproc.Event`/`EventKind`/`ParseLine`/`EncodeUserTurn`, `personastate.State`/`Load`/`Save`/`SoulPath`/`MemoryDir`/`DefaultRoot`, `soul.Choice`/`Preset`/`Presets`/`Render`/`EnsureSoul`/`ChoiceFunc`, `persona.Config`/`Controller`/`New`/`Send`/`Events`/`SessionID`/`Close`, `tui.Model`/`New`/`PersonaPort`/`personaEventMsg`/`PersonaEventMsg` are used consistently across tasks. `Config.MemoryDir` is defined in Task 7 and consumed in Task 10. `PersonaEventMsg` (exported) added in Task 10 Step 2 to let `main` construct the otherwise-unexported message. ✓
