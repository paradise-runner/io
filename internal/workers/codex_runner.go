package workers

import (
	"bufio"
	"context"
	"strings"

	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/claudeproc"
)

// CodexRunner executes worker tasks with codex exec.
type CodexRunner struct {
	CodexPath  string
	SoulPath   string
	RunCommand CommandRunner
}

// Run implements Runner.
func (r CodexRunner) Run(ctx context.Context, req Request) (Result, error) {
	rt := agentharness.Runtime{
		CodexPath:  r.CodexPath,
		SoulPath:   r.SoulPath,
		RunCommand: r.RunCommand,
	}
	out, h, err := rt.RunPrompt(ctx, agentharness.PromptRequest{
		Harness:         string(agentharness.Codex),
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		Prompt:          req.Task,
		CWD:             req.CWD,
	})
	result := parseWorkerOutput(h, out)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	return result, err
}

func parseCodexWorkerOutput(out []byte) Result {
	var result Result
	var texts []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		ev, err := agentharness.ParseLine(string(agentharness.Codex), []byte(line))
		if err != nil {
			continue
		}
		switch ev.Kind {
		case claudeproc.KindAssistantText:
			if strings.TrimSpace(ev.Text) != "" {
				texts = append(texts, ev.Text)
			}
		case claudeproc.KindResult:
			if ev.IsError && result.Error == "" {
				result.Error = "codex worker failed"
			}
		}
	}
	result.Summary = strings.TrimSpace(strings.Join(texts, "\n"))
	result.Error = strings.TrimSpace(result.Error)
	return result
}
