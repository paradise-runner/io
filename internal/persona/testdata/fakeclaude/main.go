// Command fakeclaude emulates the subset of `claude` stream-json behavior that
// io's persona controller depends on, for fast offline tests.
//
// Behavior:
//   - Prints a system/init line. If --resume <id> was passed, it echoes that id;
//     otherwise it uses "fake-session".
//   - Reads newline-delimited JSON user turns from stdin. For each, prints an
//     assistant message whose text is "echo: <user text>" and then a result line.
//   - Exits when stdin closes.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	sessionID := "fake-session"
	model := "fake-model"
	effort := ""
	args := os.Args[1:]
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) {
			sessionID = args[i+1]
		}
		if a == "--model" && i+1 < len(args) {
			model = args[i+1]
		}
		if a == "--effort" && i+1 < len(args) {
			effort = args[i+1]
		}
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	if effort != "" {
		model = model + "/" + effort
	}
	fmt.Fprintf(out, `{"type":"system","subtype":"init","session_id":%q,"model":%q}`+"\n", sessionID, model)
	out.Flush()

	type inMsg struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var m inMsg
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		text := ""
		if len(m.Message.Content) > 0 {
			text = m.Message.Content[0].Text
		}
		reply := "echo: " + text
		b, _ := json.Marshal(reply)
		fmt.Fprintf(out, `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":%s}]}}`+"\n", b)
		fmt.Fprintf(out, `{"type":"result","subtype":"success","session_id":%q,"is_error":false,"total_cost_usd":0.001}`+"\n", sessionID)
		out.Flush()
	}
}
