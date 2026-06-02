# io - Phase 3: Compaction and Dreamer

**Status:** Planned, aligned to the current Claude+Codex harness baseline.
**Last aligned:** 2026-06-02
**Builds on:** `2026-06-01-io-phase1-persona-chat-mvp.md` and
`2026-06-02-io-phase2-workers-mcp.md`

## Goal

Keep the always-up persona healthy over long use by adding:

- A reactive compaction controller that uses the existing context threshold.
- A daily dreamer that consolidates durable memories from recent conversations.

Compaction and dreaming are separate systems. Compaction shrinks live context.
Dreaming writes durable memory. Neither should pretend to do the other's job.

## Current Baseline

Relevant current code:

```text
cmd/io/app.go                       forwards persona events and owns state
internal/claudeproc/events.go       Event has InputTokens and ContextWindow
internal/persona/persona.go         exposes LastUsage()
internal/personastate/state.go      stores CompactionThreshold and DreamChatThreshold
internal/chatlog/chatlog.go         stores displayed conversation in history.jsonl
internal/tui/dialog_context.go      shows usage
internal/tui/dialog_settings.go     persists compaction threshold
internal/tui/dialog_memory.go       reads ~/.io/memory/MEMORY.md
```

Current gaps:

- No component acts on `CompactionThreshold`.
- No state fields track last dream time or chats since dream.
- No scheduler decides when the app is idle enough to dream.
- No memory curation writer exists.

## Task 1: Extend State for Dreaming

Add dreamer bookkeeping to `personastate.State`.

Proposed fields:

```go
LastDreamAt     string `json:"last_dream_at,omitempty"`
ChatsSinceDream int    `json:"chats_since_dream,omitempty"`
DreamerModel    string `json:"dreamer_model,omitempty"`
```

Implementation notes:

- Store `LastDreamAt` as RFC3339 text for JSON readability.
- Keep zero values backward compatible.
- `DreamerModel` can default to the active model for now.
- Do not remove `DreamChatThreshold`; it is already persisted and used by the
  Settings code path.

Acceptance criteria:

- Loading old `state.json` still succeeds.
- Save/load round trips new fields.
- `Normalize()` keeps existing compatibility fields intact.

## Task 2: Add Compaction Controller

Add a small controller that decides when compaction should run.

Proposed files:

```text
internal/compaction/controller.go
internal/compaction/controller_test.go
```

Suggested API:

```go
type Decision string

const (
	DecisionNone    Decision = "none"
	DecisionCompact Decision = "compact"
)

type Controller struct {
	Threshold float64
}

func (c *Controller) Observe(ev claudeproc.Event, sessionID string) Decision
func (c *Controller) Reset()
```

Responsibilities:

- Observe `claudeproc.KindResult`.
- Ignore events with missing `InputTokens` or `ContextWindow`.
- Trigger once when `InputTokens / ContextWindow >= Threshold`.
- Suppress repeated triggers for the same session and same or lower token count.
- Reset after a new session.

Acceptance criteria:

- Tests cover below threshold, above threshold, missing usage, repeated events,
  new session reset, and invalid thresholds.
- Controller has no dependency on Bubble Tea or process execution.

## Task 3: Add Persona Command Support

The controller only decides. The app needs a way to act.

Required `persona.Controller` work:

- Add a command-oriented method, for example `SendCommand(command string) error`.
- For Claude, send `/compact` through the live stream in the format the current
  Claude CLI expects.
- For Codex, return a clear unsupported error until an equivalent operation is
  verified.

Implementation notes:

- Do not overload visible user sends if the command should not be displayed as
  user-authored chat history.
- If Claude accepts slash commands only as normal user input, keep the display
  history clean by calling the persona controller directly from `app`, not
  `app.Send`.
- If compaction produces a visible assistant message, let normal event handling
  display and persist it.

Acceptance criteria:

- Claude compaction can be triggered manually in tests with a fake CLI.
- Codex compaction failure is explicit and non-fatal.
- The TUI remains usable after a failed compaction.

## Task 4: Wire Compaction into `cmd/io/app.go`

Add compaction handling at the app boundary.

App responsibilities:

- Construct a compaction controller from persisted settings.
- Feed usage result events to the controller inside the persona event forwarder.
- Trigger `persona.SendCommand("/compact")` when the controller decides.
- Add `AppController.CompactNow() error` for manual compaction.
- Add a compact status message or context-screen result when compaction is
  unsupported or fails.

TUI changes:

- Add a "compact now" action to the Context screen or Settings screen.
- Continue to show `-` style empty usage when no usage has been observed.

Acceptance criteria:

- Auto-compaction fires at the configured threshold.
- Manual compaction works for Claude.
- Codex displays a clear unsupported state instead of silently doing nothing.
- Compaction does not write a fake `you` entry to `history.jsonl`.

## Task 5: Add Dreamer Domain and Cadence Gate

Add a scheduler/gate that decides whether dreaming should run.

Proposed files:

```text
internal/dreamer/cadence.go
internal/dreamer/cadence_test.go
internal/dreamer/types.go
```

Responsibilities:

- Count completed user-originated chats.
- Run at most once per local day.
- Require `ChatsSinceDream >= DreamChatThreshold`.
- Require app idle state: no active user turn, no active worker, no compaction
  currently running.
- Return a decision only; do not perform file writes in the cadence gate.

Implementation notes:

- In current `app`, a user-authored chat starts in `Send` and completes on the
  next result event from that user turn.
- Worker result injection should not increment the user chat counter unless the
  user directly asked for that worker and the app explicitly chooses that policy.
  Start with user-authored visible turns only.
- Use injectable time in tests.

Acceptance criteria:

- Tests cover same-day suppression, threshold gating, idle gating, and counter
  reset after a successful dream.
- Existing `DreamChatThreshold` default remains 7.

## Task 6: Add Dreamer Runner

Add a runner that asks the selected harness to produce candidate memories.

Proposed files:

```text
internal/dreamer/runner.go
internal/dreamer/runner_test.go
internal/dreamer/prompts.go
```

Input:

- Recent displayed chat entries from `history.jsonl`.
- Existing `~/.io/memory/MEMORY.md` contents, if present.
- Harness/model/effort settings.

Output:

```go
type Candidate struct {
	Insight    string
	Evidence   []string
	Confidence float64
	Tags       []string
}
```

Implementation notes:

- Start with `chatlog` as the source because it is harness-neutral.
- Claude transcript JSONL can be a later enhancement; it is not a safe only
  source now that Codex is supported.
- Keep runner process execution behind an interface so default tests remain
  offline.
- The dreamer prompt should require structured JSON output.

Acceptance criteria:

- Tests parse valid candidates, reject malformed output, and handle empty
  history.
- The runner does not write memory files directly.

## Task 7: Add Memory Curation and Writer

Add deterministic curation before any memory write.

Proposed files:

```text
internal/dreamer/curator.go
internal/dreamer/curator_test.go
internal/dreamer/writer.go
internal/dreamer/writer_test.go
```

Responsibilities:

- Compare candidates with existing memory text.
- Decide `write`, `merge`, `update`, or `skip`.
- Deduplicate near-identical insights.
- Preserve evidence pointers when possible.
- Write `~/.io/memory/MEMORY.md` atomically.

Implementation notes:

- Begin with a conservative Markdown memory index. The current Memory screen
  reads this file directly.
- Do not implement full-text search in this phase unless the memory format
  actually needs it.
- Keep speculative insights clearly labeled if they are written at all.

Acceptance criteria:

- Tests cover write, merge, update, skip, duplicate candidates, contradiction
  handling, and write failures.
- Memory screen shows the updated `MEMORY.md` after a successful dream.

## Task 8: Wire Dreamer into `cmd/io/app.go`

Add background dreaming without blocking chat.

App responsibilities:

- Increment chat counters on completed user-originated turns.
- Ask the cadence gate whether a dream should run.
- Start dreamer work only when idle.
- Prevent concurrent dream runs.
- Persist `LastDreamAt` and `ChatsSinceDream` after a successful dream.
- Surface a short status only if useful; dreaming should not dominate the chat.

Implementation notes:

- Do not run the dreamer during first-run setup.
- Do not run while a persona turn is active.
- Do not run while workers are active.
- If the dreamer fails, leave counters intact so a later idle moment can retry.

Acceptance criteria:

- Dreamer can run without blocking user input.
- Failed dream does not corrupt memory or reset counters.
- Successful dream updates state and memory atomically enough to survive restart.

## Task 9: End-to-End Verification

Default suite:

```sh
go test ./...
```

Optional integration checks:

```sh
go test -tags=integration ./internal/persona
go test -tags=integration ./internal/dreamer
```

Manual checks:

- Set a low compaction threshold in Settings and verify auto-compaction behavior
  with Claude.
- Trigger manual compaction from the TUI.
- Simulate enough completed chats to cross `DreamChatThreshold`.
- Confirm the dreamer waits for idle, writes `~/.io/memory/MEMORY.md`, and the
  Memory screen reflects it.
- Confirm Codex compaction is explicitly unsupported unless a real equivalent
  command has been implemented.

## Guardrails

- Keep compaction and dreaming separate.
- Do not write internal control prompts into displayed user history.
- Do not block the TUI on dreamer work.
- Keep all process-facing dreamer behavior fakeable in the default test suite.
- Use `history.jsonl` as the initial cross-harness dream source.
- Treat Codex unsupported operations explicitly rather than hiding them.
