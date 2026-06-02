package agentharness

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/edward-champion/io/internal/claudeproc"
	"github.com/edward-champion/io/internal/codexproc"
)

// CommandRunner executes a harness CLI and returns stdout.
type CommandRunner func(ctx context.Context, bin string, args []string, cwd string) ([]byte, error)

// Runtime carries the process-level wiring shared by persona, workers, and the
// dreamer. Harness-specific behavior stays in this package.
type Runtime struct {
	Binaries map[Kind]string

	// Legacy convenience fields for the two harnesses currently exposed by the
	// CLI. Binaries takes precedence when set.
	ClaudePath string
	CodexPath  string

	SoulPath      string
	MemoryDir     string
	MCPConfigPath string
	Workdir       string
	RunCommand    CommandRunner
}

type Command struct {
	Harness Kind
	Bin     string
	Args    []string
	CWD     string
}

type StreamRequest struct {
	Harness         string
	ResumeSessionID string
	Model           string
	ReasoningEffort string
	CWD             string
}

type TurnRequest struct {
	Harness         string
	ResumeSessionID string
	Model           string
	ReasoningEffort string
	Prompt          string
	CWD             string
}

type PromptRequest struct {
	Harness         string
	Model           string
	ReasoningEffort string
	Prompt          string
	CWD             string
}

func (r Runtime) Normalize(harness, model, effort string) (Kind, string, string, error) {
	h, ok := NormalizeKind(harness)
	if !ok {
		return DefaultKind, "", "", fmt.Errorf("unknown harness %q", harness)
	}
	return h, NormalizeModel(string(h), model), NormalizeReasoningEffort(effort), nil
}

func (r Runtime) Binary(h Kind) string {
	if r.Binaries != nil {
		if bin := strings.TrimSpace(r.Binaries[h]); bin != "" {
			return bin
		}
	}
	switch h {
	case Claude:
		if strings.TrimSpace(r.ClaudePath) != "" {
			return r.ClaudePath
		}
	case Codex:
		if strings.TrimSpace(r.CodexPath) != "" {
			return r.CodexPath
		}
	}
	def := definitionByKind(h)
	return def.DefaultBinary
}

func (r Runtime) StreamCommand(req StreamRequest) (Command, error) {
	h, model, effort, err := r.Normalize(req.Harness, req.Model, req.ReasoningEffort)
	if err != nil {
		return Command{}, err
	}
	mode, _ := InteractionModeFor(string(h))
	if mode != InteractionPersistentStream {
		return Command{}, fmt.Errorf("%s does not support persistent stream personas", h)
	}
	cwd := r.cwd(req.CWD)
	switch h {
	case Claude:
		return Command{
			Harness: h,
			Bin:     r.Binary(h),
			Args: ClaudeStreamArgs(ClaudeStreamOptions{
				ResumeSessionID: req.ResumeSessionID,
				Model:           model,
				ReasoningEffort: effort,
				SoulPath:        r.SoulPath,
				MemoryDir:       r.MemoryDir,
				MCPConfigPath:   r.MCPConfigPath,
			}),
			CWD: cwd,
		}, nil
	default:
		return Command{}, fmt.Errorf("%s stream command is not wired", h)
	}
}

func (r Runtime) TurnCommand(req TurnRequest) (Command, error) {
	h, model, effort, err := r.Normalize(req.Harness, req.Model, req.ReasoningEffort)
	if err != nil {
		return Command{}, err
	}
	mode, _ := InteractionModeFor(string(h))
	if mode != InteractionExecTurns {
		return Command{}, fmt.Errorf("%s does not support exec-turn personas", h)
	}
	cwd := r.cwd(req.CWD)
	switch h {
	case Codex:
		return Command{
			Harness: h,
			Bin:     r.Binary(h),
			Args: CodexExecArgs(CodexExecOptions{
				ResumeSessionID: req.ResumeSessionID,
				Model:           model,
				ReasoningEffort: effort,
				SoulPath:        r.SoulPath,
				Workdir:         cwd,
				Prompt:          req.Prompt,
			}),
			CWD: cwd,
		}, nil
	default:
		return Command{}, fmt.Errorf("%s turn command is not wired", h)
	}
}

func (r Runtime) PromptCommand(req PromptRequest) (Command, error) {
	h, model, effort, err := r.Normalize(req.Harness, req.Model, req.ReasoningEffort)
	if err != nil {
		return Command{}, err
	}
	def, _ := DefinitionFor(string(h))
	if !def.SupportsPrompt {
		return Command{}, fmt.Errorf("%s does not support one-shot prompts", h)
	}
	cwd := r.cwd(req.CWD)
	switch h {
	case Claude:
		return Command{
			Harness: h,
			Bin:     r.Binary(h),
			Args: ClaudePromptArgs(ClaudePromptOptions{
				Model:           model,
				ReasoningEffort: effort,
				SoulPath:        r.SoulPath,
				MemoryDir:       r.MemoryDir,
				MCPConfigPath:   r.MCPConfigPath,
			}, req.Prompt),
			CWD: cwd,
		}, nil
	case Codex:
		return Command{
			Harness: h,
			Bin:     r.Binary(h),
			Args: CodexExecArgs(CodexExecOptions{
				Model:           model,
				ReasoningEffort: effort,
				SoulPath:        r.SoulPath,
				Workdir:         cwd,
				Prompt:          req.Prompt,
			}),
			CWD: cwd,
		}, nil
	default:
		return Command{}, fmt.Errorf("%s prompt command is not wired", h)
	}
}

func (r Runtime) RunPrompt(ctx context.Context, req PromptRequest) ([]byte, Kind, error) {
	cmd, err := r.PromptCommand(req)
	if err != nil {
		return nil, DefaultKind, err
	}
	run := r.RunCommand
	if run == nil {
		run = DefaultCommandRunner
	}
	out, err := run(ctx, cmd.Bin, cmd.Args, cmd.CWD)
	return out, cmd.Harness, err
}

func (r Runtime) cwd(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	return r.Workdir
}

func ParseLine(harness string, line []byte) (claudeproc.Event, error) {
	h, ok := NormalizeKind(harness)
	if !ok {
		return claudeproc.Event{}, fmt.Errorf("unknown harness %q", harness)
	}
	switch h {
	case Codex:
		return codexproc.ParseLine(line)
	case Claude:
		return claudeproc.ParseLine(line)
	default:
		return claudeproc.Event{}, fmt.Errorf("%s parser is not wired", h)
	}
}

func EncodeStreamUserTurn(harness, text string) ([]byte, error) {
	h, ok := NormalizeKind(harness)
	if !ok {
		return nil, fmt.Errorf("unknown harness %q", harness)
	}
	if !SupportsLiveCommands(string(h)) {
		return nil, fmt.Errorf("live persona commands are unsupported for %s", h)
	}
	switch h {
	case Claude:
		return claudeproc.EncodeUserTurn(text)
	default:
		return nil, fmt.Errorf("%s stream input encoder is not wired", h)
	}
}

func DefaultCommandRunner(ctx context.Context, bin string, args []string, cwd string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, msg)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}
