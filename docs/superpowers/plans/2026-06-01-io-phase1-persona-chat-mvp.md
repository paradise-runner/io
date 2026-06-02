# io - Current Implementation Plan

**Status:** Current baseline shipped; this plan tracks the remaining implementation
work from the codebase as it exists now.
**Last aligned:** 2026-06-02

This replaces the original build-from-zero Phase 1 checklist. The repository now
already contains the chat MVP, refreshed TUI, first-run setup, persisted display
history, and both Claude and Codex harness support. Future work should extend that
baseline instead of re-creating the old Phase 1 files.

## Current Baseline

`io` is a Go 1.25.2 Bubble Tea TUI with a harness-neutral persona controller.
The default harness is Claude, but Codex is also supported. Both harnesses are
normalized into `claudeproc.Event` so the TUI has one event shape.

The current app supports:

- A main TUI entrypoint in `cmd/io`.
- An offline demo entrypoint in `cmd/iodemo`.
- First-run setup inside the TUI: choose `claude` or `codex`, then choose a
  persona preset for `~/.io/SOUL.md`.
- A refreshed chat UI with message bubbles, avatar animation, toolbar screens,
  settings, context usage, memory view, new chat, and input recall.
- Persisted state under `~/.io/state.json`.
- Persisted display history under `~/.io/history.jsonl`.
- Harness-specific session ids and models so Claude and Codex can be switched
  without clobbering each other's active session.
- Claude streaming via one long-lived `claude --output-format stream-json
  --input-format stream-json` process.
- Codex turns via `codex exec --json`, with resume support when a thread id is
  available.
- Fake Claude and fake Codex binaries for offline tests.

The current app does not yet implement:

- The embedded io MCP server.
- Worker orchestration and worker status/spinner UI.
- Automatic compaction.
- Daily memory consolidation / dreaming.
- Token-by-token partial rendering.

Companion phase plans:

- [Phase 2 - Worker Orchestration and MCP](2026-06-02-io-phase2-workers-mcp.md)
- [Phase 3 - Compaction and Dreamer](2026-06-02-io-phase3-compaction-dreamer.md)

## Current File Map

```text
go.mod                                  module github.com/edward-champion/io
cmd/io/main.go                          CLI flags and Bubble Tea program startup
cmd/io/app.go                           AppController adapter and persona lifecycle
cmd/iodemo/main.go                      Offline canned-data TUI demo

internal/agentharness/harness.go        Harness/model/effort defaults and normalization
internal/claudeproc/                    Claude stream-json parser and input encoder
internal/codexproc/                     Codex JSONL parser into normalized events
internal/persona/persona.go             Harness-neutral persona controller
internal/personastate/state.go          ~/.io paths, settings, per-harness sessions
internal/chatlog/chatlog.go             Display transcript JSONL persistence
internal/soul/                          Persona presets and SOUL.md rendering
internal/tui/                           Chat UI, setup flow, controls, screens, styling

internal/persona/testdata/fakeclaude/   Fake Claude binary for tests
internal/persona/testdata/fakecodex/    Fake Codex binary for tests
```

## Current Runtime Contract

### `cmd/io`

`cmd/io` owns application wiring:

- Parse flags: `--harness`, `--model`, `--effort`, `--claude-path`,
  `--codex-path`.
- Resolve `~/.io` via `personastate.DefaultRoot()`.
- Load and normalize `personastate.State`.
- Start `app`, which starts a persona only after setup is complete.
- Create `tui.Model`, attach the `tea.Program`, and forward persona events into
  Bubble Tea.

### `cmd/io/app.go`

`app` implements `tui.AppController` and is the boundary between the TUI and the
harness layer. It is responsible for:

- Starting/restarting `persona.Controller`.
- Persisting observed session ids as soon as events contain them.
- Writing displayed `you` and `io` messages to `history.jsonl`.
- Clearing display history on new chat.
- Reading `~/.io/memory/MEMORY.md` for the Memory screen.
- Reporting last usage/cost from the persona controller.

Future features should continue to enter the TUI through `AppController` or a
small adjacent port, not by making `internal/tui` import process-controller
concretes.

### Harness Layer

`internal/persona` is now harness-neutral:

- Claude: one persistent streaming process; user turns are encoded with
  `claudeproc.EncodeUserTurn`.
- Codex: one `codex exec --json` process per turn; only one active turn is
  allowed at a time.
- Both: parsed output is normalized to `claudeproc.Event`.

Important current constraint: Claude has a live stdin stream; Codex currently
does not. Any future command APIs, compaction triggers, or worker controls must
be harness-aware.

### State

`personastate.State` contains both legacy and current fields:

- `PersonaSessionID` and `Model` are kept in sync for compatibility.
- `ClaudeSessionID` / `CodexSessionID` are the real per-harness session fields.
- `ClaudeModel` / `CodexModel` are the real per-harness model fields.
- `Harness`, `ReasoningEffort`, `CompactionThreshold`, and
  `DreamChatThreshold` are persisted settings.

Use `Normalize()`, `ActiveSessionID()`, `SetSessionIDForHarness()`,
`ActiveModel()`, and `SetActiveModel()` instead of mutating compatibility fields
directly.

## Remaining Implementation

### Task 1: Lock the Current Baseline

- [x] Keep the README aligned with the implemented CLI, state paths, and current
  feature set.
- [x] Replace the old Phase 1 checklist with this current-state plan.
- [ ] Add or update a short architecture note for the harness-neutral
  controller, if future contributors need more than this plan.
- [ ] Run `go test ./...` before starting each new feature phase.

Acceptance criteria:

- New work starts from `cmd/io`, `cmd/io/app.go`, `internal/persona`, and
  `internal/tui` as they exist now.
- No future plan asks an implementer to create files that already exist.
- The plan explicitly calls out Claude/Codex differences where they affect a
  feature.

### Task 2: Worker Orchestration

Add a harness-aware worker manager without changing the main chat contract.

Proposed files:

```text
internal/workers/manager.go
internal/workers/manager_test.go
internal/workers/runner.go
internal/workers/result_schema.go
```

Responsibilities:

- `Manager.Spawn(cwd, task) -> worker_id`.
- Track worker state: queued, running, succeeded, failed, timed out.
- Emit status events that `cmd/io/app.go` can forward into the TUI.
- On completion, inject a concise worker result back into the persona thread.
- Keep worker output structured enough for reliable result summaries.

Harness notes:

- Claude workers should use `claude -p` with JSON output and bounded turns.
- Codex workers should use `codex exec --json` when selected as the active
  harness.
- Worker process construction should live behind a runner interface so tests can
  use fakes without invoking either real CLI.

TUI integration:

- Add a compact worker status strip to the existing chrome rather than replacing
  the message view.
- Surface completed/failed workers as normal transcript messages or system-style
  status entries.
- Keep `internal/tui` process-agnostic; it should consume worker status through
  an app port.

Acceptance criteria:

- Unit tests cover spawn, status transitions, timeout, cancellation, and result
  injection.
- The default test suite remains offline.
- Manual verification covers at least one worker from the running TUI.

### Task 3: Embedded io MCP Server

Expose worker orchestration to the persona through an embedded MCP server.

Tools to expose:

- `spawn_worker(cwd, task) -> {worker_id}`
- `worker_status() -> [{worker_id, cwd, label, state}]`
- `list_projects() -> [dirs]`

Implementation guidance:

- Keep the MCP server in its own package, for example `internal/iomcp`.
- The server should call the worker manager; it should not know about the TUI.
- `cmd/io/app.go` should own MCP startup and pass the generated config path into
  `persona.Config` when the active harness supports it.
- Claude MCP wiring can use `--mcp-config`.
- Codex MCP wiring must be verified before implementation; until verified,
  Codex should continue to work without MCP-backed worker tools.

Acceptance criteria:

- Persona can request a worker via MCP and immediately get a worker id.
- Worker completion is reflected in the main chat without blocking the persona
  process.
- MCP startup/shutdown is covered by tests using a local fake worker manager.

### Task 4: Compaction Controller

The Settings screen already persists `CompactionThreshold`; implement the logic
that uses it.

Proposed files:

```text
internal/compaction/controller.go
internal/compaction/controller_test.go
```

Responsibilities:

- Observe `claudeproc.KindResult` usage events.
- Compute `InputTokens / ContextWindow` when both values are available.
- Trigger compaction once usage crosses the configured threshold.
- Provide a manual "compact now" app action for the Context or Settings screen.
- Avoid repeated compactions for the same usage window.

Harness notes:

- Claude can be compacted through a command sent to the live session.
- Codex support is not equivalent in the current controller because turns are
  one-shot. Codex compaction should be a no-op or an explicit unsupported state
  until a real command path is implemented.

Acceptance criteria:

- Threshold persistence remains unchanged.
- Context screen still works when usage fields are unavailable.
- Unit tests cover threshold crossing, missing usage, repeat suppression, and
  unsupported harness behavior.

### Task 5: Dreamer / Memory Consolidation

Implement daily consolidation as a separate subsystem from compaction.

Current setup to build on:

- `chatlog` is a reliable harness-neutral display transcript.
- `~/.io/memory/MEMORY.md` is already read by the Memory screen.
- `DreamChatThreshold` is already persisted in state.

Proposed files:

```text
internal/dreamer/dreamer.go
internal/dreamer/dreamer_test.go
internal/dreamer/prompts.go
```

Responsibilities:

- Track the last dream timestamp and post-dream chat count in state.
- Run at most once per day and only after the configured chat threshold.
- Read recent displayed conversation from `history.jsonl`.
- Ask the selected harness to produce candidate durable memories.
- Merge, deduplicate, and write curated memory updates under `~/.io/memory`.
- Keep the Memory screen read-only.

Implementation guidance:

- Start with `history.jsonl`; it works for both Claude and Codex.
- Claude transcript JSONL can be added later as a richer source, but should not
  be the only source because Codex support is now part of the current setup.
- Treat memory writes as a separate, testable function. Do not scatter file
  writes through the scheduler.

Acceptance criteria:

- Dreamer never runs while a user turn or worker is active.
- Tests cover cadence, threshold, empty history, duplicate memory candidates,
  and write failures.
- Manual verification shows the Memory screen reflecting a generated
  `MEMORY.md`.

### Task 6: Streaming and UI Polish

Token-by-token rendering is still deferred. The current parser emits complete
assistant text blocks.

Future work:

- Extend `claudeproc` to parse Claude stream deltas if the current CLI event
  shape supports them.
- Decide whether Codex can provide comparable partial events. If not, keep
  streaming behavior harness-specific and make the UI tolerate both modes.
- Add tests that render partial text without duplicating final assistant
  messages.
- Keep `chatlog` writes tied to final displayed messages, not every partial
  token.

Acceptance criteria:

- Partial rendering improves responsiveness for Claude without regressing Codex.
- Display history remains clean and non-duplicative.
- Existing bubble/content tests continue to pass.

## Test Plan

Run the normal suite after documentation or code changes:

```sh
go test ./...
```

Run the real-Claude smoke test only when Claude is installed and authenticated:

```sh
go test -tags=integration ./internal/persona
```

Manual checks for TUI-affecting work:

- `go run ./cmd/iodemo` for offline layout inspection.
- `go run ./cmd/io --harness claude` for Claude wiring.
- `go run ./cmd/io --harness codex` for Codex wiring.
- Verify setup only appears when `~/.io/SOUL.md` is missing.
- Verify `~/.io/history.jsonl` restores displayed chat on restart.

## Guardrails

- Keep `internal/tui` decoupled from concrete harness/process types.
- Keep all harness output normalized into `claudeproc.Event` unless a new
  cross-harness event type is needed.
- Add new process-facing behavior behind fakeable interfaces.
- Preserve the offline default test suite.
- Do not hardcode Claude-only assumptions into features that the current UI
  presents as harness-neutral.
- Keep compatibility fields in `personastate.State` normalized rather than
  removing them in feature work.
