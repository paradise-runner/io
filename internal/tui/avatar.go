package tui

// Expression selects io's eye glyphs, giving the avatar a little life.
type Expression int

const (
	Resting Expression = iota // ◕  calm default
	Blink                     // •  brief idle blink
	Happy                     // ♥  just after a reply
	Working                   // ✧✦★  animated while a turn is in flight
	Sleepy                    // ˘  idle for a long time
)

var workingEyes = []string{"✧", "✦", "★", "✦"}

func eyeGlyph(e Expression, frame int) string {
	switch e {
	case Blink:
		return "•"
	case Happy:
		return "♥"
	case Sleepy:
		return "˘"
	case Working:
		return workingEyes[frame%len(workingEyes)]
	default:
		return "◕"
	}
}

// Face renders io's kawaii face for an expression. frame animates Working (and
// is ignored by the static expressions).
func Face(e Expression, frame int) string {
	eye := eyeGlyph(e, frame)
	return "(" + eye + "‿" + eye + ")"
}
