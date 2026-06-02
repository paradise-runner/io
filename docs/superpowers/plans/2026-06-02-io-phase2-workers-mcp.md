# io - Phase 2: Worker Orchestration and MCP

**Status:** Implemented as the Phase 2 baseline; real-harness MCP/tool behavior
still needs manual smoke verification.
**Last aligned:** 2026-06-02
**Builds on:** `2026-06-01-io-phase1-persona-chat-mvp.md`

## Goal

Add io-managed worker delegation without changing the main chat surface. The
persona should be able to ask io to spawn background workers, the TUI should show
their lifecycle at a glance, and completed work should flow back into the main
persona thread.

This phase is not compaction, dreaming, memory editing, or token streaming.

## Current Baseline

Relevant current code:

```text
cmd/io/app.go                    owns persona lifecycle and TUI event forwarding
cmd/io/main.go                   parses harness/model/effort/path flags
internal/agentharness/           harness defaults and normalization
internal/persona/persona.go      Claude stream process and Codex one-shot turns
internal/claudeproc/             normalized event type used by the TUI
internal/codexproc/              Codex JSONL -> claudeproc.Event
internal/tui/                    chat UI, chrome, toolbar screens
```

The current architecture has two constraints that this phase must preserve:

- `internal/tui` must stay process-agnostic. It can render worker status, but it
  should not import worker runners, harness process code, or MCP server code.
- Claude has a persistent stdin stream. Codex currently runs one `codex exec
  --json` process per turn. Worker and MCP behavior must be harness-aware.

## MCP Shape

The original design said "embedded MCP over stdio", but the current app is an
alt-screen terminal UI. Its stdin/stdout are used by Bubble Tea, so they cannot
also be the persona's MCP stdio pipe.

Use this shape instead:

1. Main `io` starts a local control socket for worker operations.
2. Main `io` writes a temporary MCP config for the persona.
3. The MCP config launches the same binary in helper mode:
   `io mcp-stdio --control-socket <socket>`.
4. The helper speaks MCP over its stdio to the agent CLI and forwards tool calls
   to the main app over the control socket.
5. The main app owns the worker manager, so the TUI sees every lifecycle event.

This keeps the TUI, worker manager, and persona lifecycle in one controlling
process while still using normal stdio MCP from the agent CLI's perspective.

## Task 1: Extract Harness Command Builders

Add pure command-builder helpers so persona and workers do not drift.

Proposed files:

```text
internal/agentharness/commands.go
internal/agentharness/commands_test.go
```

Responsibilities:

- Build Claude streaming args currently assembled in `persona.startClaude`.
- Build Claude prompt-worker args for `claude -p`.
- Build Codex exec args currently assembled in `persona.codexExecFlags`.
- Keep normalization in `agentharness.NormalizeKind`, `NormalizeModel`, and
  `NormalizeReasoningEffort`.

Implementation notes:

- Move only argument construction. Do not move process ownership out of
  `internal/persona` yet.
- Preserve existing behavior for `--settings {"autoMemoryDirectory":...}`,
  `--append-system-prompt-file`, `--model`, and `--effort`.
- Codex builder should preserve `--json`, `--skip-git-repo-check`, `--model`,
  `--config model_reasoning_effort=...`, optional instructions file, and
  optional `--cd`.

Acceptance criteria:

- Existing persona tests still pass.
- Command-builder tests assert Claude and Codex args for model, effort, resume,
  workdir, SOUL path, and memory dir.
- No worker package depends on unexported `persona.Controller` helpers.

## Task 2: Add Worker Domain and Manager

Add a worker manager with fakeable runners.

Proposed files:

```text
internal/workers/types.go
internal/workers/manager.go
internal/workers/manager_test.go
internal/workers/runner.go
```

Suggested API:

```go
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateTimedOut  State = "timed_out"
	StateCanceled  State = "canceled"
)

type Request struct {
	CWD             string
	Task            string
	Label           string
	Harness         string
	Model           string
	ReasoningEffort string
	Timeout         time.Duration
}

type Status struct {
	ID         string
	CWD        string
	Label      string
	State      State
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
}

type Result struct {
	ID      string
	Summary string
	Error   string
}

type Runner interface {
	Run(ctx context.Context, req Request) (Result, error)
}
```

Responsibilities:

- `Manager.Spawn(ctx, Request) (workerID string, error)` returns immediately.
- Maintain status in memory.
- Emit status events on every lifecycle transition.
- Enforce timeout and cancellation.
- Keep a bounded history of completed workers for the status strip.

Acceptance criteria:

- Tests cover successful completion, failure, timeout, cancellation, status
  snapshots, and event ordering.
- Tests use fake runners only.
- Manager has no direct dependency on Bubble Tea or MCP.

## Task 3: Implement Harness Worker Runners

Add concrete runners that invoke the selected harness.

Proposed files:

```text
internal/workers/claude_runner.go
internal/workers/codex_runner.go
internal/workers/runner_test.go
```

Claude runner:

- Run in the target `cwd`.
- Use `claude -p` with JSON output.
- Pass model, effort, SOUL path, memory dir, and any bounded-turn/permission
  flags that are verified against the installed Claude CLI.
- Parse the final result into `workers.Result`.

Codex runner:

- Run `codex exec --json --cd <cwd>` with the selected model and effort.
- Parse agent messages and turn completion from `codexproc`.
- Return a concise final summary.

Implementation notes:

- Keep process execution behind a small test hook or runner factory so command
  construction and parsing can be tested without real CLIs.
- Do not assume Claude-only flags apply to Codex.
- Add one optional integration test per harness behind build tags if needed.

Acceptance criteria:

- Offline tests verify command construction and parser behavior.
- Real CLI tests are opt-in.
- Worker errors produce a structured failure result instead of panicking or
  silently disappearing.

## Task 4: Wire Workers into `cmd/io/app.go`

`cmd/io/app.go` should own the worker manager because it already owns persona
startup, state, and event forwarding.

Required app changes:

- Add a worker manager to `app`.
- Forward worker status events into the Bubble Tea program.
- Add an internal method to inject worker completion into the persona without
  writing it to `history.jsonl` as a user-authored message.
- Persist displayed assistant responses as today; do not persist internal worker
  injection prompts as `you`.

Suggested injection text:

```text
Worker <id> finished in <cwd>.

Task:
<task>

Result:
<summary>

Please fold this into the conversation briefly.
```

Acceptance criteria:

- Worker completion causes the persona to produce a normal visible response.
- Display history remains clean on restart.
- Worker failure is visible in the TUI and is also injected to the persona for a
  short explanation or next step.

## Task 5: Add TUI Worker Status Strip

Add a compact status strip to the existing chrome.

Proposed files:

```text
internal/tui/workers.go
internal/tui/workers_test.go
```

TUI additions:

- A `WorkerStatusEntry` value type in `internal/tui`.
- A `WorkerEventMsg(...) tea.Msg` constructor.
- `Model` stores active and recently completed worker statuses.
- Chrome renders a one-line strip such as:
  `workers  cgw#1 running  docs#2 done  api#3 failed`

Implementation notes:

- Prefer plain text states over Unicode-only symbols so narrow terminals remain
  readable.
- If there is not enough room, truncate labels first and hide completed workers
  before active workers.
- Do not add a drill-in worker view in this phase.

Acceptance criteria:

- Tests cover rendering at narrow and normal widths.
- Worker status never overlaps the input tray or toolbar.
- TUI still works when no worker manager exists, such as `cmd/iodemo`.

## Task 6: Add Parent Control Socket

Add a local IPC layer between the MCP helper and the main app.

Proposed files:

```text
internal/ioipc/server.go
internal/ioipc/client.go
internal/ioipc/protocol.go
internal/ioipc/server_test.go
```

Protocol operations:

- `spawn_worker`
- `worker_status`
- `list_projects`

Implementation notes:

- Use a Unix-domain socket on macOS/Linux under a per-run directory in `/tmp` or
  the OS temp dir.
- The server lives in the main app process and calls the worker manager.
- The client lives in MCP helper mode.
- Keep payloads JSON encoded and versioned.
- Clean up the socket on app shutdown.

Acceptance criteria:

- Tests cover spawn/status/list over the socket with a fake worker manager.
- Socket failure returns a clear MCP tool error.
- The server does not expose arbitrary process execution.

## Task 7: Add MCP Helper Mode

Add a hidden command path to the existing binary.

CLI shape:

```sh
io mcp-stdio --control-socket /path/to/io.sock
```

Proposed files:

```text
cmd/io/mcp.go
internal/iomcp/server.go
internal/iomcp/server_test.go
```

Tools:

- `spawn_worker(cwd, task) -> {worker_id}`
- `worker_status() -> [{worker_id, cwd, label, state}]`
- `list_projects() -> [dirs]`

Implementation notes:

- `cmd/io/main.go` should detect `mcp-stdio` before starting Bubble Tea.
- The MCP helper should use stdio only in helper mode.
- Main TUI mode should generate a temporary MCP config that launches this helper.
- Add `persona.Config.MCPConfigPath` and pass `--mcp-config` to Claude when set.
- Codex MCP wiring must be verified before adding it to `codex exec`. Until
  verified, keep Codex chat functional without MCP-backed worker tools.

Acceptance criteria:

- Claude persona can call `spawn_worker`.
- The call returns immediately with a worker id.
- Main TUI receives worker status events.
- MCP helper shutdown does not close or corrupt the Bubble Tea TUI.

## Task 8: End-to-End Verification

Default suite:

```sh
go test ./...
```

Optional integration checks:

```sh
go test -tags=integration ./internal/persona
go test -tags=integration ./internal/workers
```

Manual checks:

- Launch `go run ./cmd/io --harness claude`.
- Ask io to delegate a small task in a test directory.
- Confirm the status strip shows running, then done or failed.
- Confirm the result comes back into the main chat.
- Restart io and confirm worker-internal prompts were not written as displayed
  user history.

## Guardrails

- Do not make `internal/tui` know about MCP or process runners.
- Do not bind an MCP stdio server to the TUI process stdin/stdout.
- Keep Codex behavior explicit when a feature is Claude-only.
- Keep all worker process behavior fakeable in the default test suite.
- Keep worker result injection separate from user-authored history.
