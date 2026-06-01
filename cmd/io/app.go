package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/chatlog"
	"github.com/edward-champion/io/internal/claudeproc"
	"github.com/edward-champion/io/internal/persona"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
	"github.com/edward-champion/io/internal/tui"
)

// app implements tui.AppController. It owns the persona controller and io's
// persisted state, and forwards persona events into the Bubble Tea program,
// re-subscribing whenever the persona is restarted (New chat / model change).
type app struct {
	root    string
	runtime runtimeConfig

	mu   sync.Mutex
	st   personastate.State
	p    *persona.Controller
	prog *tea.Program

	setupRequired bool
}

type runtimeConfig struct {
	ClaudePath string
	CodexPath  string
}

// newApp loads state and starts the persona when setup has already completed.
func newApp(root string, st personastate.State, rt runtimeConfig) (*app, error) {
	st.Normalize()
	setupRequired, err := needsSetup(root)
	if err != nil {
		return nil, err
	}
	a := &app{root: root, st: st, runtime: rt, setupRequired: setupRequired}
	if !setupRequired {
		if err := a.startPersona(st.ActiveSessionID()); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func needsSetup(root string) (bool, error) {
	if _, err := os.Stat(personastate.SoulPath(root)); err == nil {
		return false, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	} else {
		return false, err
	}
}

// startPersona starts a persona process. resume is the session id to continue
// (empty for a fresh session). Caller holds a.mu (or is constructing).
func (a *app) startPersona(resume string) error {
	p, err := persona.New(persona.Config{
		Harness:         a.st.Harness,
		ClaudePath:      a.runtime.ClaudePath,
		CodexPath:       a.runtime.CodexPath,
		ResumeSessionID: resume,
		Model:           a.st.ActiveModel(),
		ReasoningEffort: a.st.ReasoningEffort,
		SoulPath:        personastate.SoulPath(a.root),
		MemoryDir:       personastate.MemoryDir(a.root),
		// Pin the working directory so harness session storage is deterministic
		// and io is not tied to wherever it was launched.
		Workdir: a.root,
	})
	if err != nil {
		return err
	}
	a.p = p
	if a.prog != nil {
		a.forward(p)
	}
	return nil
}

// forward pumps a persona's events into the program until its channel closes,
// logging io's replies to the display transcript on the way through.
func (a *app) forward(p *persona.Controller) {
	go func() {
		for ev := range p.Events() {
			a.mu.Lock()
			// Persist the session id as soon as it's observed (the init event),
			// so resume survives even an abrupt exit (e.g. closing the window).
			if ev.SessionID != "" && ev.SessionID != a.st.ActiveSessionID() {
				a.st.SetSessionIDForHarness(p.Harness(), ev.SessionID)
				_ = personastate.Save(a.root, &a.st)
			}
			if ev.Kind == claudeproc.KindAssistantText && strings.TrimSpace(ev.Text) != "" {
				a.writeHistory("io", ev.Text)
			}
			a.mu.Unlock()
			a.prog.Send(tui.PersonaEventMsg(ev))
		}
	}()
}

// writeHistory appends an entry to the display log. Caller holds a.mu.
func (a *app) writeHistory(role, text string) {
	_ = chatlog.Append(personastate.HistoryPath(a.root), chatlog.Entry{Role: role, Text: text})
}

// attach wires the program handle and begins forwarding the current persona.
func (a *app) attach(prog *tea.Program) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prog = prog
	if a.p != nil {
		a.forward(a.p)
	}
}

// persist captures the current session id and writes state to disk. Caller
// holds a.mu.
func (a *app) persist() error {
	if a.p == nil {
		return personastate.Save(a.root, &a.st)
	}
	if sid := a.p.SessionID(); sid != "" {
		a.st.SetSessionIDForHarness(a.p.Harness(), sid)
	}
	return personastate.Save(a.root, &a.st)
}

// --- tui.AppController ---

func (a *app) NeedsSetup() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.setupRequired
}

func (a *app) CompleteSetup(harness string, choice soul.Choice) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.st.SetHarness(harness)
	a.st.ClearActiveSessionID()
	if _, err := soul.EnsureSoul(personastate.SoulPath(a.root), func() (soul.Choice, error) {
		return choice, nil
	}); err != nil {
		return err
	}
	a.setupRequired = false
	if err := personastate.Save(a.root, &a.st); err != nil {
		return err
	}
	return a.startPersona("")
}

func (a *app) Send(text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p == nil {
		return errors.New("io setup is not complete")
	}
	a.writeHistory("you", text)
	return a.p.Send(text)
}

func (a *app) NewSession() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p == nil {
		return a.persist()
	}
	_ = a.persist()
	_ = a.p.Close()
	a.st.ClearActiveSessionID()
	_ = chatlog.Clear(personastate.HistoryPath(a.root))
	return a.startPersona("")
}

func (a *app) History() []tui.HistoryEntry {
	entries, _ := chatlog.Load(personastate.HistoryPath(a.root))
	out := make([]tui.HistoryEntry, 0, len(entries))
	for _, e := range entries {
		role := tui.RoleIO
		if e.Role == "you" {
			role = tui.RoleYou
		}
		out = append(out, tui.HistoryEntry{Role: role, Text: e.Text})
	}
	return out
}

func (a *app) SetModel(model string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.st.SetActiveModel(model)
	return a.persist()
}

func (a *app) Settings() personastate.State {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.st.Normalize()
	return a.st
}

func (a *app) SaveSettings(st personastate.State) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p != nil {
		if sid := a.p.SessionID(); sid != "" {
			st.SetSessionIDForHarness(a.p.Harness(), sid)
		}
	}
	st.Normalize()
	a.st = st
	return personastate.Save(a.root, &a.st)
}

func (a *app) ContextInfo() tui.ContextInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p == nil {
		return tui.ContextInfo{}
	}
	in, win, cost := a.p.LastUsage()
	return tui.ContextInfo{InputTokens: in, ContextWindow: win, CostUSD: cost}
}

func (a *app) MemorySummary() (string, error) {
	b, err := os.ReadFile(filepath.Join(personastate.MemoryDir(a.root), "MEMORY.md"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Close persists the session id and shuts down the persona.
func (a *app) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.persist()
	if a.p == nil {
		return nil
	}
	return a.p.Close()
}
