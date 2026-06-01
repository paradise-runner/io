// Package agentharness defines the shared agent harness choices and the
// provider-specific defaults io uses to keep harness switching predictable.
package agentharness

import "strings"

type Kind string

const (
	Claude Kind = "claude"
	Codex  Kind = "codex"

	DefaultKind            = Claude
	DefaultClaudeModel     = "sonnet"
	DefaultCodexModel      = "gpt-5.4"
	DefaultReasoningEffort = "medium"
)

var (
	harnesses    = []string{string(Claude), string(Codex)}
	claudeModels = []string{"sonnet", "opus"}
	codexModels  = []string{DefaultCodexModel, "gpt-5.5"}
	efforts      = []string{"low", DefaultReasoningEffort, "high"}
)

func NormalizeKind(s string) (Kind, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(Claude), "claude-code", "claude_code":
		return Claude, true
	case string(Codex):
		return Codex, true
	default:
		return DefaultKind, false
	}
}

func HarnessOptions() []string { return clone(harnesses) }

func DefaultModel(harness string) string {
	h, _ := NormalizeKind(harness)
	if h == Codex {
		return DefaultCodexModel
	}
	return DefaultClaudeModel
}

func ModelOptions(harness string) []string {
	h, _ := NormalizeKind(harness)
	if h == Codex {
		return clone(codexModels)
	}
	return clone(claudeModels)
}

func NormalizeModel(harness, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return DefaultModel(harness)
	}
	h, _ := NormalizeKind(harness)
	if h == Codex && (model == "sonnet" || model == "opus" || strings.HasPrefix(model, "claude-")) {
		return DefaultModel(harness)
	}
	if h == Claude && strings.HasPrefix(model, "gpt-") {
		return DefaultModel(harness)
	}
	return model
}

func EffortOptions() []string { return clone(efforts) }

func NormalizeReasoningEffort(effort string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	for _, allowed := range efforts {
		if effort == allowed {
			return effort
		}
	}
	return DefaultReasoningEffort
}

func clone(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}
