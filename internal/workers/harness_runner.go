package workers

import (
	"context"
	"strings"

	"github.com/edward-champion/io/internal/agentharness"
)

// HarnessRunner runs worker prompts through the central agent harness runtime.
type HarnessRunner struct {
	Runtime agentharness.Runtime
}

// Run implements Runner.
func (r HarnessRunner) Run(ctx context.Context, req Request) (Result, error) {
	rt := r.Runtime
	out, h, err := rt.RunPrompt(ctx, agentharness.PromptRequest{
		Harness:         req.Harness,
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		Prompt:          req.Task,
		CWD:             req.CWD,
	})
	result := parseWorkerOutput(h, out)
	if err != nil && strings.TrimSpace(result.Error) == "" {
		result.Error = err.Error()
	}
	return result, err
}

func parseWorkerOutput(h agentharness.Kind, out []byte) Result {
	if h == agentharness.Codex {
		return parseCodexWorkerOutput(out)
	}
	return parseClaudeWorkerOutput(out)
}
