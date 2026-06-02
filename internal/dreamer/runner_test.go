package dreamer

import (
	"context"
	"strings"
	"testing"

	"github.com/edward-champion/io/internal/chatlog"
)

func TestRunnerEmptyHistoryNoops(t *testing.T) {
	called := false
	r := Runner{RunCommand: func(context.Context, string, []string, string) ([]byte, error) {
		called = true
		return nil, nil
	}}
	got, err := r.Run(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if called {
		t.Fatal("empty history should not call harness")
	}
	if len(got) != 0 {
		t.Fatalf("candidates = %v, want none", got)
	}
}

func TestRunnerParsesCandidatesFromInjectedCommand(t *testing.T) {
	r := Runner{RunCommand: func(ctx context.Context, bin string, args []string, cwd string) ([]byte, error) {
		if bin != "claude" {
			t.Fatalf("bin = %q, want claude", bin)
		}
		if !strings.Contains(strings.Join(args, " "), "--output-format") {
			t.Fatalf("args missing claude output format: %v", args)
		}
		return []byte(`{"candidates":[{"insight":"User prefers concise summaries","evidence":["you: keep it short"],"confidence":0.9,"tags":["preference"]}]}`), nil
	}}
	got, err := r.Run(context.Background(), Request{
		History: []chatlog.Entry{{Role: "you", Text: "keep it short"}},
		Harness: "claude",
		Model:   "sonnet",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(got) != 1 || got[0].Insight != "User prefers concise summaries" {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestParseCandidatesFromClaudePromptJSON(t *testing.T) {
	out := []byte(`{"type":"result","result":"{\"candidates\":[{\"insight\":\"A\",\"confidence\":0.7}]}"}`)
	got, err := ParseCandidates(out)
	if err != nil {
		t.Fatalf("ParseCandidates error: %v", err)
	}
	if len(got) != 1 || got[0].Insight != "A" {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestParseCandidatesFromCodexJSONL(t *testing.T) {
	out := []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"{\"candidates\":[{\"insight\":\"B\",\"confidence\":0.8}]}"}}`)
	got, err := ParseCandidates(out)
	if err != nil {
		t.Fatalf("ParseCandidates error: %v", err)
	}
	if len(got) != 1 || got[0].Insight != "B" {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestParseCandidatesRejectsMalformedOutput(t *testing.T) {
	if _, err := ParseCandidates([]byte(`{"candidates":[`)); err == nil {
		t.Fatal("ParseCandidates should reject malformed JSON")
	}
}
