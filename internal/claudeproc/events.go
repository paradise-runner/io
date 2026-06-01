package claudeproc

// EventKind classifies a parsed stream-json line.
type EventKind int

const (
	KindUnknown EventKind = iota
	KindInit
	KindAssistantText
	KindResult
)

func (k EventKind) String() string {
	switch k {
	case KindInit:
		return "init"
	case KindAssistantText:
		return "assistant_text"
	case KindResult:
		return "result"
	default:
		return "unknown"
	}
}

// Event is the normalized, parsed form of one line of claude stream-json output.
// Only the fields relevant to a given Kind are populated.
type Event struct {
	Kind          EventKind
	SessionID     string  // KindInit, KindResult
	Model         string  // KindInit
	Text          string  // KindAssistantText (concatenated text blocks)
	IsError       bool    // KindResult
	CostUSD       float64 // KindResult
	InputTokens   int     // KindResult (usage.input_tokens)
	ContextWindow int     // KindResult (modelUsage.<model>.contextWindow)
}
