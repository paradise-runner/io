package dreamer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/claudeproc"
)

// CommandRunner executes a harness CLI and returns stdout.
type CommandRunner = agentharness.CommandRunner

// Runner asks the selected harness to produce candidate memories.
type Runner struct {
	Runtime agentharness.Runtime

	ClaudePath    string
	CodexPath     string
	SoulPath      string
	MemoryDir     string
	MCPConfigPath string
	Workdir       string
	RunCommand    CommandRunner
}

// Run executes one dream pass. Empty history is a successful no-op.
func (r Runner) Run(ctx context.Context, req Request) ([]Candidate, error) {
	if len(req.History) == 0 {
		return nil, nil
	}
	prompt := BuildPrompt(req.History, req.ExistingMemory)
	rt := r.runtime()
	out, _, err := rt.RunPrompt(ctx, agentharness.PromptRequest{
		Harness:         req.Harness,
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		Prompt:          prompt,
	})
	if err != nil {
		return nil, err
	}
	return ParseCandidates(out)
}

func (r Runner) runtime() agentharness.Runtime {
	rt := r.Runtime
	if r.ClaudePath != "" {
		rt.ClaudePath = r.ClaudePath
	}
	if r.CodexPath != "" {
		rt.CodexPath = r.CodexPath
	}
	if r.SoulPath != "" {
		rt.SoulPath = r.SoulPath
	}
	if r.MemoryDir != "" {
		rt.MemoryDir = r.MemoryDir
	}
	if r.MCPConfigPath != "" {
		rt.MCPConfigPath = r.MCPConfigPath
	}
	if r.Workdir != "" {
		rt.Workdir = r.Workdir
	}
	if r.RunCommand != nil {
		rt.RunCommand = r.RunCommand
	}
	return rt
}

type candidateEnvelope struct {
	Candidates []Candidate `json:"candidates"`
	Result     string      `json:"result"`
	Summary    string      `json:"summary"`
	Text       string      `json:"text"`
	Message    string      `json:"message"`
	Error      string      `json:"error"`
	IsError    bool        `json:"is_error"`
}

// ParseCandidates extracts candidate JSON from direct JSON, Claude prompt JSON,
// Claude stream-json text, or Codex JSONL assistant text.
func ParseCandidates(out []byte) ([]Candidate, error) {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, errors.New("dreamer output is empty")
	}
	if cs, ok, err := decodeCandidateJSON(trimmed); ok || err != nil {
		if err == nil || !strings.Contains(trimmed, "\n") || json.Valid([]byte(trimmed)) {
			return cs, err
		}
	}

	var texts []string
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if cs, ok, err := decodeCandidateJSON(line); ok || err != nil {
			return cs, err
		}
		var raw candidateEnvelope
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			if raw.Error != "" || raw.IsError {
				if raw.Error == "" {
					raw.Error = "dreamer model failed"
				}
				return nil, errors.New(raw.Error)
			}
			for _, text := range []string{raw.Result, raw.Summary, raw.Text, raw.Message} {
				if strings.TrimSpace(text) != "" {
					texts = append(texts, text)
				}
			}
		}
		if ev, err := agentharness.ParseLine(string(agentharness.Claude), []byte(line)); err == nil && ev.Kind == claudeproc.KindAssistantText {
			texts = append(texts, ev.Text)
			continue
		}
		if ev, err := agentharness.ParseLine(string(agentharness.Codex), []byte(line)); err == nil && ev.Kind == claudeproc.KindAssistantText {
			texts = append(texts, ev.Text)
		}
	}
	if len(texts) == 0 {
		return nil, errors.New("dreamer output did not contain candidate JSON")
	}
	return parseCandidateJSON(strings.Join(texts, "\n"))
}

func decodeCandidateJSON(s string) ([]Candidate, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '{' && s[0] != '[') {
		return nil, false, nil
	}
	var raw candidateEnvelope
	if err := json.Unmarshal([]byte(s), &raw); err == nil {
		if raw.Error != "" || raw.IsError {
			if raw.Error == "" {
				raw.Error = "dreamer model failed"
			}
			return nil, true, errors.New(raw.Error)
		}
		if raw.Candidates != nil {
			cs, err := normalizeCandidates(raw.Candidates)
			return cs, true, err
		}
	} else if s[0] == '{' {
		return nil, true, err
	}
	if s[0] == '{' {
		return nil, false, nil
	}
	var list []Candidate
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return nil, true, err
	}
	cs, err := normalizeCandidates(list)
	return cs, true, err
}

func parseCandidateJSON(s string) ([]Candidate, error) {
	cs, ok, err := decodeCandidateJSON(s)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("dreamer text did not contain candidate JSON")
	}
	return cs, nil
}

func normalizeCandidates(in []Candidate) ([]Candidate, error) {
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		c.Insight = strings.TrimSpace(c.Insight)
		if c.Insight == "" {
			return nil, errors.New("dreamer candidate missing insight")
		}
		if c.Confidence == 0 {
			c.Confidence = 0.5
		}
		if c.Confidence < 0 || c.Confidence > 1 {
			return nil, fmt.Errorf("dreamer candidate confidence %.2f outside 0..1", c.Confidence)
		}
		c.Evidence = trimList(c.Evidence)
		c.Tags = trimList(c.Tags)
		out = append(out, c)
	}
	return out, nil
}

func trimList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
