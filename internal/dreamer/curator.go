package dreamer

import (
	"strings"
	"unicode"
)

// Action describes what should happen with one candidate.
type Action string

const (
	ActionWrite  Action = "write"
	ActionMerge  Action = "merge"
	ActionUpdate Action = "update"
	ActionSkip   Action = "skip"
)

// CurationDecision records a deterministic memory decision.
type CurationDecision struct {
	Action    Action
	Candidate Candidate
	Reason    string
}

// Curate filters and classifies candidates before any memory write.
func Curate(existing string, candidates []Candidate) []CurationDecision {
	existingNorm := normalizeMemoryText(existing)
	var decisions []CurationDecision
	var accepted []string
	for _, c := range candidates {
		c.Insight = strings.TrimSpace(c.Insight)
		norm := normalizeMemoryText(c.Insight)
		switch {
		case norm == "":
			decisions = append(decisions, CurationDecision{Action: ActionSkip, Candidate: c, Reason: "empty insight"})
		case c.Confidence > 0 && c.Confidence < 0.5:
			decisions = append(decisions, CurationDecision{Action: ActionSkip, Candidate: c, Reason: "low confidence"})
		case isContradiction(c):
			decisions = append(decisions, CurationDecision{Action: ActionSkip, Candidate: c, Reason: "contradiction"})
		case duplicateOf(norm, accepted):
			decisions = append(decisions, CurationDecision{Action: ActionSkip, Candidate: c, Reason: "duplicate candidate"})
		case strings.Contains(existingNorm, norm):
			if hasNewEvidence(existing, c.Evidence) {
				decisions = append(decisions, CurationDecision{Action: ActionMerge, Candidate: c, Reason: "existing insight with new evidence"})
			} else {
				decisions = append(decisions, CurationDecision{Action: ActionSkip, Candidate: c, Reason: "already in memory"})
			}
		case hasTag(c, "update"):
			decisions = append(decisions, CurationDecision{Action: ActionUpdate, Candidate: c, Reason: "tagged update"})
			accepted = append(accepted, norm)
		default:
			decisions = append(decisions, CurationDecision{Action: ActionWrite, Candidate: c, Reason: "new memory"})
			accepted = append(accepted, norm)
		}
	}
	return decisions
}

func hasNewEvidence(existing string, evidence []string) bool {
	existingNorm := normalizeMemoryText(existing)
	for _, ev := range evidence {
		if evNorm := normalizeMemoryText(ev); evNorm != "" && !strings.Contains(existingNorm, evNorm) {
			return true
		}
	}
	return false
}

func isContradiction(c Candidate) bool {
	if hasTag(c, "contradiction") || hasTag(c, "conflict") {
		return true
	}
	return strings.Contains(normalizeMemoryText(c.Insight), "contradicts")
}

func hasTag(c Candidate, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, tag := range c.Tags {
		if strings.ToLower(strings.TrimSpace(tag)) == want {
			return true
		}
	}
	return false
}

func duplicateOf(norm string, accepted []string) bool {
	for _, prior := range accepted {
		if norm == prior || similarity(norm, prior) >= 0.92 {
			return true
		}
	}
	return false
}

func similarity(a, b string) float64 {
	at := tokenSet(a)
	bt := tokenSet(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	intersect := 0
	for tok := range at {
		if bt[tok] {
			intersect++
		}
	}
	union := len(at) + len(bt) - intersect
	return float64(intersect) / float64(union)
}

func tokenSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, tok := range strings.Fields(s) {
		out[tok] = true
	}
	return out
}

func normalizeMemoryText(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
			continue
		}
		space = true
	}
	return strings.TrimSpace(b.String())
}
