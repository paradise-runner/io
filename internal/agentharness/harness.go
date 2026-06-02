// Package agentharness defines the shared agent harness choices and every
// provider-specific interaction shape io knows how to drive.
package agentharness

import "strings"

type Kind string

type InteractionMode string

const (
	Claude Kind = "claude"
	Codex  Kind = "codex"

	DefaultKind            = Claude
	DefaultClaudeModel     = "sonnet"
	DefaultCodexModel      = "gpt-5.4"
	DefaultReasoningEffort = "medium"
)

const (
	InteractionPersistentStream InteractionMode = "persistent_stream"
	InteractionExecTurns        InteractionMode = "exec_turns"
)

// Definition is io's central registry record for one supported agent harness.
// Adding a harness should start here, then add its command and parser hook in
// this package.
type Definition struct {
	Kind                 Kind
	DisplayName          string
	DefaultBinary        string
	Aliases              []string
	DefaultModel         string
	Models               []string
	RejectModelValues    []string
	RejectModelPrefixes  []string
	InteractionMode      InteractionMode
	SupportsPrompt       bool
	SupportsLiveCommands bool
}

var (
	definitions = []Definition{
		{
			Kind:                 Claude,
			DisplayName:          "Claude Code",
			DefaultBinary:        "claude",
			Aliases:              []string{"claude-code", "claude_code", "claudecode"},
			DefaultModel:         DefaultClaudeModel,
			Models:               []string{DefaultClaudeModel, "opus"},
			RejectModelPrefixes:  []string{"gpt-"},
			InteractionMode:      InteractionPersistentStream,
			SupportsPrompt:       true,
			SupportsLiveCommands: true,
		},
		{
			Kind:                Codex,
			DisplayName:         "Codex",
			DefaultBinary:       "codex",
			DefaultModel:        DefaultCodexModel,
			Models:              []string{DefaultCodexModel, "gpt-5.5"},
			RejectModelValues:   []string{"sonnet", "opus"},
			RejectModelPrefixes: []string{"claude-"},
			InteractionMode:     InteractionExecTurns,
			SupportsPrompt:      true,
		},
	}
	efforts = []string{"low", DefaultReasoningEffort, "high"}
)

func NormalizeKind(s string) (Kind, bool) {
	def, ok := DefinitionFor(s)
	if !ok {
		return DefaultKind, false
	}
	return def.Kind, true
}

func Definitions() []Definition {
	out := make([]Definition, 0, len(definitions))
	for _, def := range definitions {
		out = append(out, cloneDefinition(def))
	}
	return out
}

func DefinitionFor(harness string) (Definition, bool) {
	key := normalizeToken(harness)
	if key == "" {
		return definitionByKind(DefaultKind), true
	}
	for _, def := range definitions {
		if key == normalizeToken(string(def.Kind)) || key == normalizeToken(def.DisplayName) {
			return cloneDefinition(def), true
		}
		for _, alias := range def.Aliases {
			if key == normalizeToken(alias) {
				return cloneDefinition(def), true
			}
		}
	}
	return Definition{}, false
}

func HarnessOptions() []string {
	out := make([]string, 0, len(definitions))
	for _, def := range definitions {
		out = append(out, string(def.Kind))
	}
	return out
}

func DefaultModel(harness string) string {
	def, ok := DefinitionFor(harness)
	if !ok {
		def = definitionByKind(DefaultKind)
	}
	return def.DefaultModel
}

func ModelOptions(harness string) []string {
	def, ok := DefinitionFor(harness)
	if !ok {
		def = definitionByKind(DefaultKind)
	}
	return clone(def.Models)
}

func NormalizeModel(harness, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return DefaultModel(harness)
	}
	def, ok := DefinitionFor(harness)
	if !ok {
		def = definitionByKind(DefaultKind)
	}
	for _, rejected := range def.RejectModelValues {
		if model == rejected {
			return def.DefaultModel
		}
	}
	for _, prefix := range def.RejectModelPrefixes {
		if strings.HasPrefix(model, prefix) {
			return def.DefaultModel
		}
	}
	return model
}

func InteractionModeFor(harness string) (InteractionMode, bool) {
	def, ok := DefinitionFor(harness)
	if !ok {
		return "", false
	}
	return def.InteractionMode, true
}

func SupportsLiveCommands(harness string) bool {
	def, ok := DefinitionFor(harness)
	return ok && def.SupportsLiveCommands
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

func definitionByKind(kind Kind) Definition {
	for _, def := range definitions {
		if def.Kind == kind {
			return cloneDefinition(def)
		}
	}
	return Definition{}
}

func cloneDefinition(def Definition) Definition {
	def.Aliases = clone(def.Aliases)
	def.Models = clone(def.Models)
	def.RejectModelValues = clone(def.RejectModelValues)
	def.RejectModelPrefixes = clone(def.RejectModelPrefixes)
	return def
}

func normalizeToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func clone(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}
