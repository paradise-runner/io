package claudeproc

import "encoding/json"

type userInput struct {
	Type    string       `json:"type"`
	Message userInputMsg `json:"message"`
}

type userInputMsg struct {
	Role    string       `json:"role"`
	Content []rawContent `json:"content"`
}

// EncodeUserTurn encodes a user message as a single newline-terminated line of
// stream-json suitable for writing to a claude process's stdin when it was
// started with --input-format stream-json. The returned bytes include the
// trailing newline that terminates the line.
func EncodeUserTurn(text string) ([]byte, error) {
	in := userInput{
		Type: "user",
		Message: userInputMsg{
			Role:    "user",
			Content: []rawContent{{Type: "text", Text: text}},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
