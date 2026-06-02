package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/chatlog"
	"github.com/edward-champion/io/internal/claudeproc"
	"github.com/edward-champion/io/internal/compaction"
	"github.com/edward-champion/io/internal/dreamer"
	"github.com/edward-champion/io/internal/ioipc"
	"github.com/edward-champion/io/internal/persona"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
	"github.com/edward-champion/io/internal/tui"
	"github.com/edward-champion/io/internal/workers"
)

// app implements tui.AppController. It owns the persona controller and io's
// persisted state, and forwards persona events into the Bubble Tea program,
// re-subscribing whenever the persona is restarted (New chat / model change).
type app struct {
	root    string
	runtime runtimeConfig

	mu      sync.Mutex
	st      personastate.State
	p       *persona.Controller
	prog    *tea.Program
	workers *workers.Manager
	control *ioipc.Server

	compactor     *compaction.Controller
	compacting    bool
	compactStatus string

	userTurnActive bool
	dreamCadence   dreamer.Cadence
	dreamRunner    dreamer.Runner
	dreaming       bool
	dreamStatus    string

	controlDir    string
	mcpConfigPath string

	setupRequired bool
}

type runtimeConfig = agentharness.Runtime

const defaultWorkerTimeout = 15 * time.Minute
const defaultDreamTimeout = 10 * time.Minute

// newApp loads state and starts the persona when setup has already completed.
func newApp(root string, st personastate.State, rt runtimeConfig) (*app, error) {
	st.Normalize()
	setupRequired, err := needsSetup(root)
	if err != nil {
		return nil, err
	}
	a := &app{
		root:          root,
		st:            st,
		runtime:       rt,
		setupRequired: setupRequired,
		compactor:     &compaction.Controller{Threshold: st.CompactionThreshold},
		dreamCadence:  dreamer.Cadence{},
	}
	if err := a.startControlSocket(); err != nil {
		return nil, err
	}
	a.workers = workers.NewManager(workers.HarnessRunner{Runtime: a.harnessRuntime()})
	a.dreamRunner = dreamer.Runner{Runtime: a.harnessRuntime()}
	if !setupRequired {
		if err := a.startPersona(st.ActiveSessionID()); err != nil {
			_ = a.closeControlSocket()
			return nil, err
		}
	}
	a.forwardWorkers()
	return a, nil
}

func (a *app) harnessRuntime() agentharness.Runtime {
	rt := a.runtime
	rt.SoulPath = personastate.SoulPath(a.root)
	rt.MemoryDir = personastate.MemoryDir(a.root)
	rt.MCPConfigPath = a.mcpConfigPath
	rt.Workdir = a.root
	return rt
}

func (a *app) startControlSocket() error {
	dir, err := os.MkdirTemp("", "io-control-*")
	if err != nil {
		return err
	}
	socket := filepath.Join(dir, "io.sock")
	server, err := ioipc.Listen(socket, a)
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	a.controlDir = dir
	a.control = server
	if err := a.writeMCPConfig(socket); err != nil {
		_ = a.closeControlSocket()
		return err
	}
	go func() { _ = server.Serve() }()
	return nil
}

func (a *app) writeMCPConfig(socket string) error {
	bin, err := os.Executable()
	if err != nil || bin == "" {
		bin = "io"
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"io": map[string]any{
				"command": bin,
				"args":    []string{"mcp-stdio", "--control-socket", socket},
			},
		},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	a.mcpConfigPath = filepath.Join(a.controlDir, "mcp.json")
	return os.WriteFile(a.mcpConfigPath, b, 0o600)
}

func (a *app) closeControlSocket() error {
	if a.control != nil {
		_ = a.control.Close()
	}
	if a.controlDir != "" {
		return os.RemoveAll(a.controlDir)
	}
	return nil
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
		ResumeSessionID: resume,
		Model:           a.st.ActiveModel(),
		ReasoningEffort: a.st.ReasoningEffort,
		// Pin the working directory so harness session storage is deterministic
		// and io is not tied to wherever it was launched.
		Workdir: a.root,
		Runtime: a.harnessRuntime(),
	})
	if err != nil {
		return err
	}
	a.p = p
	if a.compactor != nil {
		a.compactor.Reset()
	}
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
			var compactTarget *persona.Controller
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
			if ev.Kind == claudeproc.KindResult {
				if a.compacting {
					a.compacting = false
					a.compactStatus = "compaction finished"
				}
				if a.userTurnActive {
					a.userTurnActive = false
					a.st.ChatsSinceDream++
					_ = personastate.Save(a.root, &a.st)
				}
				if a.compactor != nil && a.p == p && !a.compacting &&
					a.compactor.Observe(ev, p.SessionID()) == compaction.DecisionCompact {
					a.compacting = true
					a.compactStatus = "auto-compaction requested"
					compactTarget = p
				}
			}
			prog := a.prog
			a.mu.Unlock()
			if compactTarget != nil {
				if err := compactTarget.SendCommand("/compact"); err != nil {
					a.mu.Lock()
					a.compacting = false
					a.compactStatus = "compaction failed: " + err.Error()
					a.mu.Unlock()
				}
			}
			if ev.Kind == claudeproc.KindResult {
				a.maybeStartDream()
			}
			if prog != nil {
				prog.Send(tui.PersonaEventMsg(ev))
			}
		}
	}()
}

func (a *app) forwardWorkers() {
	go func() {
		for ev := range a.workers.Events() {
			a.mu.Lock()
			prog := a.prog
			a.mu.Unlock()
			if prog != nil {
				prog.Send(tui.WorkerEventMsg(toTUIWorkerStatus(ev.Status)))
			}
			if ev.Status.State.Terminal() {
				a.injectWorkerResult(ev)
				a.maybeStartDream()
			}
		}
	}()
}

func toTUIWorkerStatus(st workers.Status) tui.WorkerStatusEntry {
	return tui.WorkerStatusEntry{
		ID:    st.ID,
		CWD:   st.CWD,
		Label: st.Label,
		State: string(st.State),
		Error: st.Error,
	}
}

func (a *app) injectWorkerResult(ev workers.Event) {
	text := workerInjectionText(ev)
	a.mu.Lock()
	p := a.p
	a.mu.Unlock()
	if p == nil {
		return
	}
	_ = p.Send(text)
}

func workerInjectionText(ev workers.Event) string {
	summary := ""
	if ev.Result != nil {
		summary = strings.TrimSpace(ev.Result.Summary)
		if summary == "" {
			summary = strings.TrimSpace(ev.Result.Error)
		}
	}
	if summary == "" {
		summary = strings.TrimSpace(ev.Status.Error)
	}
	if summary == "" {
		summary = string(ev.Status.State)
	}
	return "Worker " + ev.Status.ID + " finished in " + ev.Status.CWD + ".\n\n" +
		"Task:\n" + ev.Request.Task + "\n\n" +
		"Result:\n" + summary + "\n\n" +
		"Please fold this into the conversation briefly."
}

// writeHistory appends an entry to the display log. Caller holds a.mu.
func (a *app) writeHistory(role, text string) {
	_ = chatlog.Append(personastate.HistoryPath(a.root), chatlog.Entry{Role: role, Text: text})
}

// attach wires the program handle and begins forwarding the current persona.
func (a *app) attach(prog *tea.Program) {
	a.mu.Lock()
	a.prog = prog
	if a.p != nil {
		a.forward(a.p)
	}
	var statuses []workers.Status
	if a.workers != nil {
		statuses = a.workers.Snapshot()
	}
	a.mu.Unlock()
	for _, st := range statuses {
		prog.Send(tui.WorkerEventMsg(toTUIWorkerStatus(st)))
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
	if err := a.p.Send(text); err != nil {
		return err
	}
	a.writeHistory("you", text)
	a.userTurnActive = true
	return nil
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
	a.userTurnActive = false
	a.compacting = false
	if a.compactor != nil {
		a.compactor.Reset()
	}
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
	if a.compactor == nil {
		a.compactor = &compaction.Controller{}
	}
	a.compactor.Threshold = a.st.CompactionThreshold
	a.compactor.Reset()
	return personastate.Save(a.root, &a.st)
}

func (a *app) ContextInfo() tui.ContextInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	compactStatus := a.compactStatus
	if a.compacting {
		compactStatus = "compaction running"
	}
	dreamStatus := a.dreamStatus
	if a.dreaming {
		dreamStatus = "dreaming"
	}
	if a.p == nil {
		return tui.ContextInfo{Compaction: compactStatus, Dreaming: dreamStatus}
	}
	in, win, cost := a.p.LastUsage()
	return tui.ContextInfo{InputTokens: in, ContextWindow: win, CostUSD: cost, Compaction: compactStatus, Dreaming: dreamStatus}
}

func (a *app) CompactNow() error {
	a.mu.Lock()
	if a.p == nil {
		a.mu.Unlock()
		return errors.New("io setup is not complete")
	}
	if a.compacting {
		a.mu.Unlock()
		return errors.New("compaction already running")
	}
	p := a.p
	a.compacting = true
	a.compactStatus = "manual compaction requested"
	a.mu.Unlock()

	if err := p.SendCommand("/compact"); err != nil {
		a.mu.Lock()
		a.compacting = false
		a.compactStatus = "compaction failed: " + err.Error()
		a.mu.Unlock()
		return err
	}
	return nil
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

func (a *app) spawnWorker(ctx context.Context, req workers.Request) (string, error) {
	a.mu.Lock()
	st := a.st
	st.Normalize()
	a.mu.Unlock()

	if strings.TrimSpace(req.CWD) == "" {
		req.CWD = a.root
	}
	if strings.TrimSpace(req.Label) == "" {
		req.Label = filepath.Base(req.CWD)
	}
	if strings.TrimSpace(req.Harness) == "" {
		req.Harness = st.Harness
	}
	if strings.TrimSpace(req.Model) == "" {
		req.Model = st.ActiveModel()
	}
	if strings.TrimSpace(req.ReasoningEffort) == "" {
		req.ReasoningEffort = st.ReasoningEffort
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultWorkerTimeout
	}
	return a.workers.Spawn(ctx, req)
}

func (a *app) SpawnWorker(ctx context.Context, req ioipc.SpawnWorkerRequest) (string, error) {
	return a.spawnWorker(ctx, workers.Request{
		CWD:             req.CWD,
		Task:            req.Task,
		Label:           req.Label,
		Harness:         req.Harness,
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		Timeout:         req.Timeout,
	})
}

func (a *app) WorkerStatus(context.Context) ([]ioipc.WorkerStatus, error) {
	snapshot := a.workers.Snapshot()
	out := make([]ioipc.WorkerStatus, 0, len(snapshot))
	for _, st := range snapshot {
		out = append(out, ioipc.WorkerStatus{
			ID:         st.ID,
			CWD:        st.CWD,
			Label:      st.Label,
			State:      string(st.State),
			StartedAt:  st.StartedAt,
			FinishedAt: st.FinishedAt,
			Error:      st.Error,
		})
	}
	return out, nil
}

func (a *app) ListProjects(context.Context) ([]string, error) {
	return []string{a.root}, nil
}

type dreamJob struct {
	root         string
	state        personastate.State
	runner       dreamer.Runner
	chatsAtStart int
}

func (a *app) maybeStartDream() {
	a.mu.Lock()
	job, ok := a.prepareDreamLocked()
	a.mu.Unlock()
	if ok {
		go a.runDream(job)
	}
}

func (a *app) prepareDreamLocked() (dreamJob, bool) {
	st := dreamer.State{
		LastDreamAt:        a.st.LastDreamAt,
		ChatsSinceDream:    a.st.ChatsSinceDream,
		DreamChatThreshold: a.st.DreamChatThreshold,
	}
	idle := dreamer.IdleState{
		UserTurnActive: a.userTurnActive,
		ActiveWorkers:  activeWorkerCount(a.workers),
		Compacting:     a.compacting,
		Dreaming:       a.dreaming,
		SetupRequired:  a.setupRequired,
	}
	if a.dreamCadence.ShouldRun(st, idle) != dreamer.DecisionDream {
		return dreamJob{}, false
	}
	a.dreaming = true
	a.dreamStatus = "dreaming"
	state := a.st
	state.Normalize()
	return dreamJob{
		root:         a.root,
		state:        state,
		runner:       a.dreamRunner,
		chatsAtStart: a.st.ChatsSinceDream,
	}, true
}

func activeWorkerCount(m *workers.Manager) int {
	if m == nil {
		return 0
	}
	count := 0
	for _, st := range m.Snapshot() {
		if !st.State.Terminal() {
			count++
		}
	}
	return count
}

func (a *app) runDream(job dreamJob) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDreamTimeout)
	defer cancel()

	err := a.executeDream(ctx, job)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dreaming = false
	if err != nil {
		a.dreamStatus = "dream failed: " + err.Error()
		return
	}
	ds := dreamer.State{}
	a.dreamCadence.MarkSucceeded(&ds)
	a.st.LastDreamAt = ds.LastDreamAt
	a.st.ChatsSinceDream -= job.chatsAtStart
	if a.st.ChatsSinceDream < 0 {
		a.st.ChatsSinceDream = 0
	}
	a.dreamStatus = "dream complete"
	_ = personastate.Save(a.root, &a.st)
}

func (a *app) executeDream(ctx context.Context, job dreamJob) error {
	history, err := chatlog.Load(personastate.HistoryPath(job.root))
	if err != nil {
		return err
	}
	existing, err := readMemory(job.root)
	if err != nil {
		return err
	}
	model := job.state.DreamerModel
	if strings.TrimSpace(model) == "" {
		model = job.state.ActiveModel()
	}
	candidates, err := job.runner.Run(ctx, dreamer.Request{
		History:         history,
		ExistingMemory:  existing,
		Harness:         job.state.Harness,
		Model:           model,
		ReasoningEffort: job.state.ReasoningEffort,
	})
	if err != nil {
		return err
	}
	decisions := dreamer.Curate(existing, candidates)
	return dreamer.WriteAtomic(filepath.Join(personastate.MemoryDir(job.root), "MEMORY.md"), existing, decisions)
}

func readMemory(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(personastate.MemoryDir(root), "MEMORY.md"))
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
	_ = a.closeControlSocket()
	if a.p == nil {
		return nil
	}
	return a.p.Close()
}
