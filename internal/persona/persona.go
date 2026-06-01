// Package persona owns the selected agent harness process for the io persona.
// It feeds user turns to the harness and emits parsed events on a channel.
package persona

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/claudeproc"
	"github.com/edward-champion/io/internal/codexproc"
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
	// Workdir is the process working directory (default: current dir).
	Workdir string
}

// Controller manages the selected harness persona process.
type Controller struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	events  chan claudeproc.Event
	harness string
	cfg     Config

	mu         sync.RWMutex
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
	h, ok := agentharness.NormalizeKind(cfg.Harness)
	if !ok {
		return nil, fmt.Errorf("unknown harness %q", cfg.Harness)
	}
	cfg.Harness = string(h)
	cfg.Model = agentharness.NormalizeModel(cfg.Harness, cfg.Model)
	cfg.ReasoningEffort = agentharness.NormalizeReasoningEffort(cfg.ReasoningEffort)
	c := &Controller{
		events:    make(chan claudeproc.Event, 64),
		harness:   cfg.Harness,
		cfg:       cfg,
		sessionID: cfg.ResumeSessionID,
	}
	if h == agentharness.Codex {
		return c, nil
	}
	if err := c.startClaude(cfg); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Controller) startClaude(cfg Config) error {
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
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.ReasoningEffort != "" {
		args = append(args, "--effort", cfg.ReasoningEffort)
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
	go c.readLoop(stdout)
	return nil
}

func (c *Controller) readLoop(stdout io.Reader) {
	defer c.closeEvents()
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
		c.record(ev)
		if ev.Kind == claudeproc.KindUnknown {
			continue
		}
		c.emit(ev)
	}
}

func (c *Controller) runCodexTurn(text string) {
	defer func() {
		c.mu.Lock()
		c.running = false
		c.cmd = nil
		c.mu.Unlock()
	}()

	cmd := exec.Command(c.codexBin(), c.codexArgs(text)...)
	if c.cfg.Workdir != "" {
		cmd.Dir = c.cfg.Workdir
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
		ev, err := codexproc.ParseLine(line)
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

func (c *Controller) codexBin() string {
	if c.cfg.CodexPath != "" {
		return c.cfg.CodexPath
	}
	return "codex"
}

func (c *Controller) codexArgs(text string) []string {
	sessionID := c.SessionID()
	if sessionID == "" {
		args := []string{"exec"}
		args = append(args, c.codexExecFlags(true)...)
		return append(args, text)
	}
	args := []string{"exec", "resume"}
	args = append(args, c.codexExecFlags(false)...)
	args = append(args, sessionID, text)
	return args
}

func (c *Controller) codexExecFlags(includeCD bool) []string {
	args := []string{
		"--json",
		"--skip-git-repo-check",
		"--model", c.cfg.Model,
		"--config", "model_reasoning_effort=" + tomlString(c.cfg.ReasoningEffort),
	}
	if c.cfg.SoulPath != "" {
		args = append(args, "--config", "model_instructions_file="+tomlString(c.cfg.SoulPath))
	}
	if includeCD && c.cfg.Workdir != "" {
		args = append(args, "--cd", c.cfg.Workdir)
	}
	return args
}

func tomlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
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
	if c.harness == string(agentharness.Codex) {
		c.mu.Lock()
		if c.running {
			c.mu.Unlock()
			return errors.New("codex turn already running")
		}
		c.running = true
		c.mu.Unlock()
		go c.runCodexTurn(text)
		return nil
	}
	line, err := claudeproc.EncodeUserTurn(text)
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(line)
	return err
}

// Close closes stdin and waits for the process to exit.
func (c *Controller) Close() error {
	if c.harness == string(agentharness.Codex) {
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
