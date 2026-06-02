package agentharness

import (
	"encoding/json"
	"fmt"
)

// ClaudeStreamOptions configures the long-lived Claude stream-json persona
// process.
type ClaudeStreamOptions struct {
	ResumeSessionID string
	Model           string
	ReasoningEffort string
	SoulPath        string
	MemoryDir       string
	MCPConfigPath   string
}

// ClaudePromptOptions configures a one-shot Claude prompt worker.
type ClaudePromptOptions struct {
	Model           string
	ReasoningEffort string
	SoulPath        string
	MemoryDir       string
	MCPConfigPath   string
}

// CodexExecOptions configures a codex exec turn.
type CodexExecOptions struct {
	ResumeSessionID string
	Model           string
	ReasoningEffort string
	SoulPath        string
	Workdir         string
	Prompt          string
}

// ClaudeStreamArgs builds args for the persistent Claude persona process.
func ClaudeStreamArgs(opts ClaudeStreamOptions) []string {
	args := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	}
	if opts.ResumeSessionID != "" {
		args = append(args, "--resume", opts.ResumeSessionID)
	}
	return appendClaudeCommonArgs(args, opts.Model, opts.ReasoningEffort, opts.SoulPath, opts.MemoryDir, opts.MCPConfigPath)
}

// ClaudePromptArgs builds args for a one-shot Claude worker prompt.
func ClaudePromptArgs(opts ClaudePromptOptions, prompt string) []string {
	args := []string{"-p", prompt, "--output-format", "json"}
	return appendClaudeCommonArgs(args, opts.Model, opts.ReasoningEffort, opts.SoulPath, opts.MemoryDir, opts.MCPConfigPath)
}

func appendClaudeCommonArgs(args []string, model, effort, soulPath, memoryDir, mcpConfigPath string) []string {
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	if soulPath != "" {
		args = append(args, "--append-system-prompt-file", soulPath)
	}
	if memoryDir != "" {
		args = append(args, "--settings", fmt.Sprintf(`{"autoMemoryDirectory":%q}`, memoryDir))
	}
	if mcpConfigPath != "" {
		args = append(args, "--mcp-config", mcpConfigPath)
	}
	return args
}

// CodexExecArgs builds args for a codex exec turn, preserving the initial-turn
// and resume shapes used by the persona controller.
func CodexExecArgs(opts CodexExecOptions) []string {
	if opts.ResumeSessionID == "" {
		args := []string{"exec"}
		args = append(args, CodexExecFlags(opts, true)...)
		return append(args, opts.Prompt)
	}
	args := []string{"exec", "resume"}
	args = append(args, CodexExecFlags(opts, false)...)
	args = append(args, opts.ResumeSessionID, opts.Prompt)
	return args
}

// CodexExecFlags builds the shared flag block for codex exec.
func CodexExecFlags(opts CodexExecOptions, includeCD bool) []string {
	args := []string{
		"--json",
		"--skip-git-repo-check",
		"--model", opts.Model,
		"--config", "model_reasoning_effort=" + TOMLString(opts.ReasoningEffort),
	}
	if opts.SoulPath != "" {
		args = append(args, "--config", "model_instructions_file="+TOMLString(opts.SoulPath))
	}
	if includeCD && opts.Workdir != "" {
		args = append(args, "--cd", opts.Workdir)
	}
	return args
}

// TOMLString renders s as a TOML-compatible quoted string for CLI config args.
func TOMLString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
