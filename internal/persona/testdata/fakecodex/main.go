// Command fakecodex emulates the subset of `codex exec --json` that io's
// persona controller depends on, for fast offline tests.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	threadID := "fake-codex-session"
	if i := index(args, "resume"); i >= 0 {
		if id := firstPositional(args[i+1:]); id != "" {
			threadID = id
		}
	}
	model := flagValue(args, "--model", "-m")
	effort := configValue(args, "model_reasoning_effort")
	prompt := lastPositional(args)

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	fmt.Fprintf(out, `{"type":"thread.started","thread_id":%q,"model":%q}`+"\n", threadID, model)
	fmt.Fprintln(out, `{"type":"turn.started"}`)
	text := fmt.Sprintf("model=%s effort=%s echo: %s", model, effort, prompt)
	b, _ := json.Marshal(text)
	fmt.Fprintf(out, `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":%s}}`+"\n", b)
	fmt.Fprintf(out, `{"type":"turn.completed","thread_id":%q,"usage":{"input_tokens":42,"output_tokens":7,"reasoning_output_tokens":1}}`+"\n", threadID)
}

func index(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

func flagValue(args []string, names ...string) string {
	for i, arg := range args {
		for _, name := range names {
			if arg == name && i+1 < len(args) {
				return args[i+1]
			}
			if strings.HasPrefix(arg, name+"=") {
				return strings.TrimPrefix(arg, name+"=")
			}
		}
	}
	return ""
}

func configValue(args []string, key string) string {
	for i, arg := range args {
		if (arg == "--config" || arg == "-c") && i+1 < len(args) {
			if v, ok := splitConfig(args[i+1], key); ok {
				return v
			}
		}
	}
	return ""
}

func splitConfig(s, key string) (string, bool) {
	prefix := key + "="
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	var v string
	if err := json.Unmarshal([]byte(strings.TrimPrefix(s, prefix)), &v); err == nil {
		return v, true
	}
	return strings.TrimPrefix(s, prefix), true
}

func lastPositional(args []string) string {
	last := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if takesValue(arg) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") || arg == "exec" || arg == "resume" {
			continue
		}
		last = arg
	}
	return last
}

func firstPositional(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if takesValue(arg) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func takesValue(arg string) bool {
	switch arg {
	case "--model", "-m", "--config", "-c", "--cd", "-C":
		return true
	default:
		return false
	}
}
