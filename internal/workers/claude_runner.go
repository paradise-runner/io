package workers

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"

	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/claudeproc"
)

// ClaudeRunner executes worker tasks with claude -p.
type ClaudeRunner struct {
	ClaudePath    string
	SoulPath      string
	MemoryDir     string
	MCPConfigPath string
	RunCommand    CommandRunner
}

// Run implements Runner.
func (r ClaudeRunner) Run(ctx context.Context, req Request) (Result, error) {
	rt := agentharness.Runtime{
		ClaudePath:    r.ClaudePath,
		SoulPath:      r.SoulPath,
		MemoryDir:     r.MemoryDir,
		MCPConfigPath: r.MCPConfigPath,
		RunCommand:    r.RunCommand,
	}
	out, h, err := rt.RunPrompt(ctx, agentharness.PromptRequest{
		Harness:         string(agentharness.Claude),
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

type claudePromptJSON struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
	Message string `json:"message"`
	Error   string `json:"error"`
	IsError bool   `json:"is_error"`
}

func parseClaudeWorkerOutput(out []byte) Result {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return Result{}
	}

	var result Result
	var texts []string
	sc := bufio.NewScanner(strings.NewReader(trimmed))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if ev, err := agentharness.ParseLine(string(agentharness.Claude), []byte(line)); err == nil {
			switch ev.Kind {
			case claudeproc.KindAssistantText:
				if strings.TrimSpace(ev.Text) != "" {
					texts = append(texts, ev.Text)
				}
				continue
			case claudeproc.KindResult:
				if ev.IsError && result.Error == "" {
					result.Error = "claude worker failed"
				}
				continue
			}
		}

		var raw claudePromptJSON
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if raw.Result != "" {
			result.Summary = raw.Result
		} else if raw.Summary != "" {
			result.Summary = raw.Summary
		} else if raw.Text != "" {
			result.Summary = raw.Text
		} else if raw.Message != "" {
			result.Summary = raw.Message
		}
		if raw.Error != "" {
			result.Error = raw.Error
		} else if raw.IsError && result.Error == "" {
			result.Error = "claude worker failed"
		}
	}
	if strings.TrimSpace(result.Summary) == "" && len(texts) > 0 {
		result.Summary = strings.Join(texts, "\n")
	}
	if strings.TrimSpace(result.Summary) == "" && result.Error == "" {
		result.Summary = trimmed
	}
	result.Summary = strings.TrimSpace(result.Summary)
	result.Error = strings.TrimSpace(result.Error)
	return result
}
