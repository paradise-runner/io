# io — UI Refresh (Pastel Dream Kawaii Chat)

**Status:** Design approved, pre-implementation
**Date:** 2026-06-01
**Author:** edward.champion (with Claude)
**Builds on:** `2026-06-01-io-personal-assistant-tui-design.md` (Phase 1 shipped)

## Summary

Redesign io's TUI from the current flat `you`/`io` transcript into a friendly,
colorful, **text-message-style chat**: two-sided pastel bubbles, an expressive
animated kawaii avatar for io, structured content (code/tables) that breaks out
into readable panels, and a row of **90s-retro beveled buttons** that open modal
dialogs (Settings, New chat, Context, Memory). Plus two small behavior changes:
io replies in **brief, chat-sized messages**, and the default model is **sonnet**.

This is a refresh of the `tui` package with small, well-bounded extensions to
`claudeproc`, `persona`, `personastate`, and `soul`. io's *voice* stays
per-persona; the kawaii is in the UI, not forced into io's words.

## Goals

- A two-sided iMessage-style bubble chat (your messages right/pink, io's
  left/mint with avatar).
- Long/structured content (fenced code, markdown tables) breaks out of bubbles
  into full-width monospace panels instead of cramming into a narrow box.
- A footer bar of 90s-retro beveled buttons, activated by **mouse click or
  Ctrl-hotkey**, each opening a centered modal dialog.
- Four dialogs: Settings, New chat (confirm), Context (usage readout), Memory
  (read-only view of the memory store).
- An **expressive animated avatar**: io's eyes cycle through glyphs (resting,
  blink, hearts, working sparkles) for whimsy and as a state indicator.
- io replies briefly (a SOUL.md baseline instruction); default model is sonnet.

## Non-Goals

- Changing io's per-persona voice/personality (kawaii is UI-only).
- Implementing the compaction controller or dreamer (those remain Phase 3); the
  Settings dialog only *persists* the compaction threshold for now.
- Worker/spinner-header orchestration (that is the separate Phase 2 plan). The
  button bar and spinner header coexist later; this refresh does not build
  workers.
- Token-by-token streaming (still renders complete assistant messages per turn).

## Background / Current State

Phase 1 renders a flat transcript in `internal/tui/model.go`: bold `you`/`io`
labels, a `viewport` of plain lines, a `textarea` input. `claudeproc` parses
`system/init`, `assistant`, and `result` lines into `Event`s; `persona` runs the
persistent `claude` process; `personastate` persists session id + settings;
`soul` renders `SOUL.md`. All of these stay; this refresh changes how the
transcript is rendered and adds the button/dialog layer, plus small fields.

## Architecture

The `tui` package grows from one file into focused units, each with one
responsibility and a pure, testable core. An `AppController` interface
(implemented in `cmd/io`) gives the TUI the app-level actions dialogs need,
keeping `tui` decoupled from `persona`/`personastate` concretes.

```
internal/tui/
  theme.go      Pastel Dream palette + all lipgloss styles (avatar, bubbles,
                panels, buttons, dialogs). Single source of color truth.
  avatar.go     Expressive avatar: eye-glyph state machine + animation tick.
  bubbles.go    renderBubble(role, text, width) — two-sided pastel bubbles.
  content.go    splitSegments(text) — prose vs code/table; segment rendering.
  buttons.go    Footer button bar: definitions, bevel rendering, mouse
                hit-testing, hotkey map.
  dialog.go     Dialog interface + modal overlay compositor.
  dialogs/      settings.go, context.go, memory.go, confirm.go (one each).
  model.go      Orchestrates transcript, input, viewport, buttons, active
                dialog, mouse + key routing.
  port.go       AppController interface (the TUI's view of the app).

cmd/io/app.go   AppController implementation: wraps persona.Controller +
                personastate, handles Send/NewSession/SetModel/ContextInfo/
                MemorySummary and persona restart + event re-subscription.
```

### Components

#### theme.go
- **Does:** defines the Pastel Dream palette and every lipgloss style used in the
  UI. io bubble = mint `#A8E6CF`; your bubble = soft pink `#FFB3D9`; accents /
  panel frame / button bevel = lavender `#C8A2E0`; dialog frame = lavender on a
  slightly raised background. Beveled button look uses block glyphs
  (`▕ ▏ ▁ ▔`) with a bright top-left and dark bottom-right edge.
- **Interface:** exported style vars / a `Theme` struct of `lipgloss.Style`s.
- **Depends on:** lipgloss only.

#### avatar.go
- **Does:** renders io's face `(<L><mouth><R>)` where the eyes are chosen by an
  `Expression`. Expressions: `Resting` (`◕ ◕`), `Blink` (`• •`), `Happy`
  (`♥ ♥`), `Working` (cycles `✧ ✦ ★ ✦`), `Sleepy` (`˘ ˘`). A `Tick()` advances
  an internal frame for idle blinks (occasional) and Working animation.
- **Interface:** `type Expression int`; `func Face(e Expression, frame int) string`
  (pure); the model owns the current Expression + frame and calls `Face`.
- **Depends on:** nothing (pure string building). Animation driven by a
  `tea.Tick` in the model.
- **State mapping:** `Resting` by default; `Working` while a turn is in flight
  (between Send and the turn's `result`); `Happy` for ~2s right after a reply,
  then back to `Resting`; random `Blink` frame on the idle tick.

#### bubbles.go
- **Does:** `renderBubble(role Role, text string, maxWidth int) string` — wraps
  prose to a bubble capped at ~60% of `maxWidth`, rounds the border, colors per
  role, right-aligns `you` / left-aligns `io`, and renders io's avatar + `io`
  label in the left gutter.
- **Interface:** pure function returning a styled multi-line string.
- **Depends on:** theme, lipgloss, avatar (for the io gutter face).

#### content.go
- **Does:** `splitSegments(text string) []Segment` where
  `Segment{Kind: Prose|Code|Table, Text, Lang}`. Detects fenced code blocks
  (```) and GitHub-style markdown tables (lines with `|` + a `---` separator
  row). `renderSegment(seg, maxWidth)` renders Prose via `renderBubble` and
  Code/Table as a full-width, monospace, lavender-framed panel labeled with the
  io avatar.
- **Interface:** two pure functions.
- **Depends on:** theme, bubbles, lipgloss.

#### buttons.go
- **Does:** defines the four buttons (`Label`, `Icon`, `Hotkey`, `Action`),
  renders the beveled bar, and provides `hitTest(x int) (Action, bool)` mapping a
  mouse x-coordinate to a button. Hotkeys: `^S` Settings, `^N` New, `^O`
  Context, `^R` Memory.
- **Interface:** `Bar` struct with `View() string`, `HitTest(x) (Action,bool)`,
  `HotkeyAction(key) (Action,bool)`.
- **Depends on:** theme, lipgloss.
- **Note:** `^N`/`^O`/`^R`/`^S` are read as `tea.KeyMsg` with `Ctrl` modifier;
  `^M` is avoided (it is Enter in terminals), which is why Memory uses `^R`.

#### dialog.go + dialogs/
- **Does:** a `Dialog` interface (`Update(tea.Msg) (Dialog, tea.Cmd)`,
  `View(width,height) string`, both with Esc-to-close handled by the model). The
  model holds an `active Dialog` (nil = none) and renders it centered over a
  dimmed transcript. Concrete dialogs:
  - **settings.go** — fields: model (sonnet/opus toggle), persona (cycle
    presets), compaction threshold (slider 0.50–0.95). On save, calls
    `AppController.SetModel` / persists via `SaveSettings`; notes that model &
    persona apply on next New chat / relaunch.
  - **context.go** — read-only: last-turn input tokens, context window, percent
    used, session cost. Pulls from `AppController.ContextInfo()`.
  - **memory.go** — read-only scrollable view of `AppController.MemorySummary()`
    (contents of `~/.io/memory/MEMORY.md`, or a friendly empty state).
  - **confirm.go** — generic yes/no used by New chat ("Start a fresh chat? io
    keeps its memories. ♡"). On confirm, calls `AppController.NewSession()`.
- **Interface:** `Dialog`; each concrete type constructed with the data/port it
  needs.
- **Depends on:** theme, lipgloss, port (AppController).

#### port.go (AppController)
The TUI's only view of the application. Implemented by `cmd/io/app.go`, stubbed
in tests.
```go
type AppController interface {
    Send(text string) error
    NewSession() error                 // fresh persona session; memory kept
    SetModel(model string) error       // persist; applies on next session
    Settings() personastate.State      // current settings snapshot
    SaveSettings(personastate.State) error
    ContextInfo() ContextInfo          // last-turn usage + cost
    MemorySummary() (string, error)    // ~/.io/memory/MEMORY.md or empty state
}

type ContextInfo struct {
    InputTokens   int
    ContextWindow int
    CostUSD       float64
}
```

#### model.go
- **Does:** owns `[]Segment` transcript (built from events via `content`), the
  `textarea`, `viewport`, button `Bar`, current avatar `Expression`+frame, and
  optional `active Dialog`. Routing: if a dialog is open, msgs go to it (Esc
  closes); otherwise Enter sends, Ctrl-hotkeys/mouse clicks open dialogs, the
  avatar tick animates the face. On a `personaEventMsg` of kind result it sets
  Happy + refreshes ContextInfo; on Send it sets Working.
- **Depends on:** all the above + port.

### cmd/io/app.go
- **Does:** implements `AppController`. Wraps the current `persona.Controller`
  and `personastate`. `NewSession` closes the persona and starts a fresh one
  (no `--resume`), generating a new session id; `SetModel` persists model and
  marks it for the next session; `ContextInfo` returns the last `result` usage
  it observed. Because restarting the persona swaps the events channel, `app.go`
  owns the goroutine that forwards persona events into the `tea.Program` and
  re-subscribes after a restart.
- **Depends on:** persona, personastate, claudeproc, bubbletea program handle.

## Small backend extensions

### claudeproc
Extend the `result` Event with usage fields parsed from `usage` /
`modelUsage[model]`:
- `InputTokens int` (from `usage.input_tokens`)
- `ContextWindow int` (from `modelUsage.<model>.contextWindow`)
- `CostUSD` already exists.
These feed the Context dialog. Unknown/missing fields default to 0.

### persona
- Add `Config.Model string`; when non-empty, pass `--model <model>`.
- Track the most recent `result` usage (input tokens, context window, cost) and
  expose `LastUsage() (input, window int, cost float64)`.
- No restart method on the controller itself; `cmd/io/app.go` restarts by
  `Close()` + `New(...)` (the controller is cheap to recreate).

### personastate
- Add `Model string` to `State`, defaulting to `"sonnet"` in `Load` when empty.

### soul
- Append a baseline **brevity** block to every rendered `SOUL.md`, after the
  persona/style sections:
  > ## Chat style
  > You're in a text-message chat. Reply in brief, chat-sized messages — usually
  > 1–3 short sentences. Don't dump large tables, long lists, or big code blocks
  > unless explicitly asked; offer a short summary and let the user ask for more.
- io's persona voice is unchanged; this only governs length/format.

## Data Flow

1. User types → Enter → `model` appends a `you` prose segment, calls
   `AppController.Send`, sets avatar `Working`.
2. `cmd/io/app.go` forwards `claude` events into the program as `personaEventMsg`.
3. On `KindAssistantText`, `model` runs `splitSegments` and appends bubbles +
   breakout panels for io; on `KindResult` it records usage (for Context),
   sets avatar `Happy` (2s), and returns to `Resting`.
4. Button click/hotkey → `model` opens the matching `Dialog`; dialog interacts
   via `AppController`; Esc closes.
5. Avatar `tea.Tick` advances blink/working animation frames.

## Error Handling & Edges

- **Persona restart (New chat / model switch) fails:** show an inline io system
  bubble ("couldn't start a fresh chat — still on the old one") and keep the
  current session.
- **Memory store missing/empty:** Memory dialog shows a friendly empty state
  ("io hasn't saved any memories yet ♡").
- **No result observed yet:** Context dialog shows "—" for usage until the first
  turn completes.
- **Very long single prose line / narrow terminal:** bubble width clamps to a
  sane minimum; word-wrap never produces negative widths.
- **Mouse disabled / unsupported terminal:** Ctrl-hotkeys still operate every
  button; mouse is additive, never required.

## Testing

- **Pure functions (string assertions):** `avatar.Face` per expression/frame;
  `renderBubble` alignment + width cap + avatar gutter; `content.splitSegments`
  on prose-only, fenced-code, table, and mixed inputs; `content.renderSegment`
  panel vs bubble; `buttons.HitTest` x-ranges and `HotkeyAction` mapping.
- **Dialog Update logic:** drive each dialog with `tea.Msg`s against a stub
  `AppController`; assert it calls `SetModel`/`NewSession`/`SaveSettings`
  correctly and closes on Esc.
- **Model routing:** with a stub `AppController`, assert a Ctrl-hotkey opens the
  right dialog, Enter sends + appends a `you` segment + sets Working, a result
  event sets Happy + refreshes ContextInfo, and that a click x maps to the
  expected action.
- **claudeproc:** extend parser tests for the new usage fields.
- **Existing tests:** persona/tui tests updated for the `Model` field and new
  rendering; the real-claude integration test is unaffected.

## Open Questions / To Confirm

None outstanding. Decisions locked: two-sided bubbles; prose-bubbles +
breakout-panels; four buttons (Settings/New/Context/Memory) via mouse +
Ctrl-hotkeys; Pastel Dream palette with `(◕‿◕)` avatar; expressive animated eyes;
io voice stays per-persona; brevity baseline in SOUL.md; default model sonnet.

## Future / Out of scope

- Wiring the compaction threshold to an actual controller (Phase 3).
- The worker spinner header coexisting with the button bar (Phase 2).
- Editable memory from the Memory dialog (read-only for now).
- Theme switcher (Vaporwave / Soft Sanrio palettes) — palette is centralized in
  `theme.go`, so adding alternates later is cheap.
