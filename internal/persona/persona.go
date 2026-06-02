// Package persona owns the selected agent harness process for the io persona.
// It feeds user turns to the harness and emits parsed events on a channel.
package persona

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/claudeproc"
)

// Config configures a persona Controller.
type Config struct {
	// Harness selects the agent CLI to drive ("claude" or "codex").
	Harness string
	// ClaudePath is the path to the claude binary (default "claude" if empty).
	ClaudePath string
	// CodexPath is the path to the codex binary (default "codex" if empty).
	CodexPath string
	// ResumeSessionID, if non-empty, resumes that session via --resume.
	ResumeSessionID string
	// Model, if non-empty, is passed via the harness's model flag.
	Model string
	// ReasoningEffort is passed through the harness's effort setting.
	ReasoningEffort string
	// SoulPath, if non-empty, is passed through the harness's persona prompt flag.
	SoulPath string
	// MemoryDir, if non-empty, is passed to harnesses with native memory-dir support.
	MemoryDir string
	// MCPConfigPath, if non-empty, is passed to harnesses that support MCP config.
	MCPConfigPath string
	// Workdir is the process working directory (default: current dir).
	Workdir string
	// Runtime centralizes harness binary paths and command construction. The
	// fields above override matching Runtime fields when set.
	Runtime agentharness.Runtime
}

// Controller manages the selected harness persona process.
type Controller struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	events  chan claudeproc.Event
	harness string
	mode    agentharness.InteractionMode
	cfg     Config
	runtime agentharness.Runtime

	mu         sync.RWMutex
	writeMu    sync.Mutex
	sessionID  string
	lastInput  int
	lastWindow int
	lastCost   float64
	running    bool

	eventMu      sync.Mutex
	eventsClosed bool
}

// New starts the persona process and begins reading its events.
func New(cfg Config) (*Controller, error) {
	rt := cfg.runtime()
	h, model, effort, err := rt.Normalize(cfg.Harness, cfg.Model, cfg.ReasoningEffort)
	if err != nil {
		return nil, err
	}
	mode, _ := agentharness.InteractionModeFor(string(h))
	cfg.Harness = string(h)
	cfg.Model = model
	cfg.ReasoningEffort = effort
	c := &Controller{
		events:    make(chan claudeproc.Event, 64),
		harness:   cfg.Harness,
		mode:      mode,
		cfg:       cfg,
		runtime:   rt,
		sessionID: cfg.ResumeSessionID,
	}
	if mode == agentharness.InteractionExecTurns {
		return c, nil
	}
	if err := c.startPersistent(cfg); err != nil {
		return nil, err
	}
	return c, nil
}

func (cfg Config) runtime() agentharness.Runtime {
	rt := cfg.Runtime
	if cfg.ClaudePath != "" {
		rt.ClaudePath = cfg.ClaudePath
	}
	if cfg.CodexPath != "" {
		rt.CodexPath = cfg.CodexPath
	}
	if cfg.SoulPath != "" {
		rt.SoulPath = cfg.SoulPath
	}
	if cfg.MemoryDir != "" {
		rt.MemoryDir = cfg.MemoryDir
	}
	if cfg.MCPConfigPath != "" {
		rt.MCPConfigPath = cfg.MCPConfigPath
	}
	if cfg.Workdir != "" {
		rt.Workdir = cfg.Workdir
	}
	return rt
}

func (c *Controller) startPersistent(cfg Config) error {
	hcmd, err := c.runtime.StreamCommand(agentharness.StreamRequest{
		Harness:         cfg.Harness,
		ResumeSessionID: cfg.ResumeSessionID,
		Model:           cfg.Model,
		ReasoningEffort: cfg.ReasoningEffort,
		CWD:             cfg.Workdir,
	})
	if err != nil {
		return err
	}

	cmd := exec.Command(hcmd.Bin, hcmd.Args...)
	if hcmd.CWD != "" {
		cmd.Dir = hcmd.CWD
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	c.cmd = cmd
	c.stdin = stdin
	go c.readLoop(hcmd.Harness, stdout)
	return nil
}

func (c *Controller) readLoop(harness agentharness.Kind, stdout io.Reader) {
	defer c.closeEvents()
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		ev, err := agentharness.ParseLine(string(harness), line)
		if err != nil {
			continue // ignore unparseable lines in Phase 1
		}
		c.record(ev)
		if ev.Kind == claudeproc.KindUnknown {
			continue
		}
		c.emit(ev)
	}
}

func (c *Controller) runTurn(text string) {
	defer func() {
		c.mu.Lock()
		c.running = false
		c.cmd = nil
		c.mu.Unlock()
	}()

	hcmd, err := c.runtime.TurnCommand(agentharness.TurnRequest{
		Harness:         c.cfg.Harness,
		ResumeSessionID: c.SessionID(),
		Model:           c.cfg.Model,
		ReasoningEffort: c.cfg.ReasoningEffort,
		Prompt:          text,
		CWD:             c.cfg.Workdir,
	})
	if err != nil {
		c.emit(claudeproc.Event{Kind: claudeproc.KindResult, IsError: true})
		return
	}

	cmd := exec.Command(hcmd.Bin, hcmd.Args...)
	if hcmd.CWD != "" {
		cmd.Dir = hcmd.CWD
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.emit(claudeproc.Event{Kind: claudeproc.KindResult, IsError: true})
		return
	}
	if err := cmd.Start(); err != nil {
		c.emit(claudeproc.Event{Kind: claudeproc.KindResult, IsError: true})
		return
	}
	c.mu.Lock()
	c.cmd = cmd
	c.mu.Unlock()

	sawResult := false
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		ev, err := agentharness.ParseLine(string(hcmd.Harness), line)
		if err != nil || ev.Kind == claudeproc.KindUnknown {
			continue
		}
		if ev.Kind == claudeproc.KindResult {
			sawResult = true
		}
		c.record(ev)
		c.emit(ev)
	}
	if err := cmd.Wait(); err != nil && !sawResult {
		c.emit(claudeproc.Event{Kind: claudeproc.KindResult, IsError: true})
	}
}

func (c *Controller) emit(ev claudeproc.Event) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	if c.eventsClosed {
		return
	}
	c.events <- ev
}

func (c *Controller) closeEvents() {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	if c.eventsClosed {
		return
	}
	c.eventsClosed = true
	close(c.events)
}

func (c *Controller) record(ev claudeproc.Event) {
	if ev.Kind == claudeproc.KindInit || (ev.Kind == claudeproc.KindResult && ev.SessionID != "") {
		c.mu.Lock()
		c.sessionID = ev.SessionID
		c.mu.Unlock()
	}
	if ev.Kind == claudeproc.KindResult {
		c.mu.Lock()
		if ev.InputTokens > 0 {
			c.lastInput = ev.InputTokens
		}
		if ev.ContextWindow > 0 {
			c.lastWindow = ev.ContextWindow
		}
		c.lastCost = ev.CostUSD
		c.mu.Unlock()
	}
}

// Events returns the channel of parsed persona events. It is closed when the
// process exits.
func (c *Controller) Events() <-chan claudeproc.Event { return c.events }

// Harness returns the agent CLI backing this controller.
func (c *Controller) Harness() string { return c.harness }

// SessionID returns the most recently observed session id (empty until init).
func (c *Controller) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// LastUsage returns the most recent observed input-token count, context window,
// and cumulative session cost (all zero until the first turn completes).
func (c *Controller) LastUsage() (inputTokens, contextWindow int, costUSD float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastInput, c.lastWindow, c.lastCost
}

// Send writes a user turn to the persona process.
func (c *Controller) Send(text string) error {
	if c.mode == agentharness.InteractionExecTurns {
		c.mu.Lock()
		if c.running {
			c.mu.Unlock()
			return fmt.Errorf("%s turn already running", c.harness)
		}
		c.running = true
		c.mu.Unlock()
		go c.runTurn(text)
		return nil
	}
	return c.sendStreamInput(text)
}

// SendCommand writes an internal command to the live persona without adding a
// user-authored history entry. Only persistent harnesses can support this.
func (c *Controller) SendCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("persona command is empty")
	}
	if !agentharness.SupportsLiveCommands(c.harness) {
		return fmt.Errorf("live persona command %q is unsupported for %s", command, c.harness)
	}
	return c.sendStreamInput(command)
}

func (c *Controller) sendStreamInput(text string) error {
	line, err := agentharness.EncodeStreamUserTurn(c.harness, text)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.stdin == nil {
		return fmt.Errorf("%s persona stdin is not available", c.harness)
	}
	_, err = c.stdin.Write(line)
	return err
}

// Close closes stdin and waits for the process to exit.
func (c *Controller) Close() error {
	if c.mode == agentharness.InteractionExecTurns {
		c.mu.RLock()
		cmd := c.cmd
		c.mu.RUnlock()
		defer c.closeEvents()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			return cmd.Wait()
		}
		return nil
	}
	_ = c.stdin.Close()
	return c.cmd.Wait()
}
