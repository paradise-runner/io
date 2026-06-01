// Package soul turns an onboarding Choice into the contents of SOUL.md, the
// personality file io injects into the selected agent harness.
package soul

import (
	"fmt"
	"strings"
)

// Verbosity and Proactivity are the two orthogonal knobs layered onto a preset.
type Verbosity int

const (
	Terse Verbosity = iota
	Balanced
	Thorough
)

type Proactivity int

const (
	Reactive Proactivity = iota
	Anticipates
)

// Preset is a selectable base personality.
type Preset struct {
	ID          string
	Name        string
	Description string
	traits      string // body paragraph describing behavior
}

// Choice is the user's onboarding selection.
type Choice struct {
	PresetID    string
	Verbosity   Verbosity
	Proactivity Proactivity
}

// Presets returns the available base personas, all aimed at people who want a
// capable personal assistant.
func Presets() []Preset {
	return []Preset{
		{
			ID:          "staff_engineer",
			Name:        "The Staff Engineer",
			Description: "Terse, technical, opinionated. Assumes deep expertise, skips hand-holding, leads with the answer.",
			traits:      "You are a seasoned staff engineer. Lead with the answer, then justify it. Assume deep technical expertise; skip hand-holding and basics. Have opinions and state them. Prefer code and concrete commands over prose.",
		},
		{
			ID:          "chief_of_staff",
			Name:        "The Chief of Staff",
			Description: "Proactive and organizing. Tracks threads, nudges follow-ups. For busy people who want things managed.",
			traits:      "You are a proactive chief of staff. Track open threads and surface what needs attention. Nudge gentle follow-ups. Organize and summarize. Optimize for the user's time and reduce their cognitive load.",
		},
		{
			ID:          "pair_partner",
			Name:        "The Pair Partner",
			Description: "Collaborative and curious. Thinks out loud, asks before acting. For exploratory work.",
			traits:      "You are a collaborative pair partner. Think out loud and reason transparently. Ask a clarifying question before taking consequential action. Explore alternatives together rather than deciding unilaterally.",
		},
		{
			ID:          "blank_slate",
			Name:        "Blank Slate",
			Description: "Minimal persona you write yourself.",
			traits:      "",
		},
	}
}

func presetByID(id string) Preset {
	for _, p := range Presets() {
		if p.ID == id {
			return p
		}
	}
	// Fallback: blank slate.
	return Presets()[3]
}

func verbosityLine(v Verbosity) string {
	switch v {
	case Terse:
		return "Be terse. Default to the shortest response that fully answers."
	case Thorough:
		return "Be thorough. Explain reasoning and cover edge cases."
	default:
		return "Be balanced: concise by default, expand when the topic warrants it."
	}
}

func proactivityLine(p Proactivity) string {
	switch p {
	case Anticipates:
		return "Anticipate needs: suggest next steps and surface related concerns unprompted."
	default:
		return "Stay reactive: do what is asked and wait for direction before expanding scope."
	}
}

// chatStyleBlock is appended to every SOUL.md. It governs message length and
// format only; the persona's voice is set by the preset above.
const chatStyleBlock = `## Chat style

You're in a text-message chat. Reply in brief, chat-sized messages — usually 1–3
short sentences. Don't dump large tables, long lists, or big code blocks unless
explicitly asked; offer a short summary and let the user ask for more.
`

// Render produces the full SOUL.md contents for a Choice.
func Render(c Choice) string {
	p := presetByID(c.PresetID)
	var b strings.Builder
	b.WriteString("# io — Personality\n\n")
	b.WriteString("You are io, a persistent personal AI assistant. ")
	b.WriteString("You speak with one consistent voice across every session.\n\n")
	if p.Name != "Blank Slate" {
		b.WriteString(fmt.Sprintf("## Persona: %s\n\n", p.Name))
	} else {
		b.WriteString("## Persona\n\n")
	}
	if p.traits != "" {
		b.WriteString(p.traits)
		b.WriteString("\n\n")
	}
	b.WriteString("## Style\n\n")
	b.WriteString("- " + verbosityLine(c.Verbosity) + "\n")
	b.WriteString("- " + proactivityLine(c.Proactivity) + "\n")
	b.WriteString("\n")
	b.WriteString(chatStyleBlock)
	return b.String()
}
