# io

`io` is a personal AI assistant TUI written in Go with Bubble Tea. It gives a
persistent assistant persona a dedicated terminal interface, stores its state
under `~/.io`, and drives an installed agent CLI instead of calling a model API
directly.

The current implementation supports Claude Code and Codex CLI harnesses, first
run persona setup, resumable sessions, local display history, model/effort
settings, context usage readouts, and a read-only memory view.

![io TUI example](io-example.png)

## Status

This repository contains the chat MVP, refreshed TUI, and Phase 2 worker
orchestration/MCP plumbing. Design notes in `docs/superpowers/` describe the
remaining direction for automatic compaction and daily memory consolidation; the
settings screen stores those thresholds for future use.

## Requirements

- Go matching `go.mod` (`go 1.25.2`)
- One or both supported agent CLIs installed and authenticated:
  - `claude` for the Claude harness
  - `codex` for the Codex harness

## Run

Start the TUI from the repository root:

```sh
go run ./cmd/io
```

On first run, `io` asks which harness to use and which personality preset to
write into `~/.io/SOUL.md`. After setup, it starts the selected harness and
resumes the saved session on later launches.

For an offline UI-only demo that does not require Claude or Codex:

```sh
go run ./cmd/iodemo
```

## CLI Options

```sh
go run ./cmd/io --harness codex --model gpt-5.4 --effort medium
```

| Flag | Description |
| --- | --- |
| `-harness`, `-agent-harness` | Agent harness: `claude` or `codex`. |
| `-model`, `-agent-model` | Model override for the selected harness. |
| `-effort`, `-agent-effort` | Reasoning effort: `low`, `medium`, or `high`. |
| `-claude-path` | Path to a specific `claude` binary. |
| `-codex-path` | Path to a specific `codex` binary. |

Defaults are defined in `internal/agentharness`: Claude is the default harness,
Claude defaults to `sonnet`, Codex defaults to `gpt-5.4`, and reasoning effort
defaults to `medium`.

## Controls

Main chat:

- `Enter`: send the current message
- `Ctrl+C`: quit
- `Up` / `Down`: recall sent messages when the input is single-line
- `PageUp` / `PageDown` or `Ctrl+U` / `Ctrl+D`: scroll chat history
- Mouse wheel: scroll chat history

Toolbar screens:

- `Ctrl+S`: Settings
- `Ctrl+N`: New chat
- `Ctrl+O`: Context usage
- `Ctrl+R`: Memory

Settings:

- `Up` / `Down` / `Tab`: move between fields
- `Left` / `Right`: change the selected field
- `Enter`: save
- `Esc`: return to chat

## Persistence

`io` stores its own state under `~/.io`:

| Path | Purpose |
| --- | --- |
| `~/.io/SOUL.md` | Generated and editable assistant personality prompt. |
| `~/.io/state.json` | Selected harness, model, effort, thresholds, and session IDs. |
| `~/.io/history.jsonl` | Display transcript used to restore the TUI chat history. |
| `~/.io/memory/` | Harness-backed assistant memory directory. |

Claude sessions are run as a persistent streaming process. Codex turns are run
through `codex exec --json` and resume the stored thread ID when available.
Claude sessions also receive a temporary MCP config for io's worker tools; Codex
MCP wiring is intentionally left disabled until verified.

## Development

Run the normal test suite:

```sh
go test ./...
```

Run the real-Claude smoke test when `claude` is installed and authenticated:

```sh
go test -tags=integration ./internal/persona
```

Most harness-facing tests use fake Claude and Codex binaries from
`internal/persona/testdata/`, so the default suite is fast and offline.

## Project Layout

| Path | Description |
| --- | --- |
| `cmd/io` | Main `io` application entrypoint and TUI controller adapter. |
| `cmd/iodemo` | Offline canned-data TUI demo. |
| `internal/agentharness` | Harness choices, model defaults, and normalization. |
| `internal/claudeproc` | Parser and input encoder for Claude stream-json. |
| `internal/codexproc` | Parser for Codex JSONL output. |
| `internal/ioipc` | Local Unix-socket control protocol for worker operations. |
| `internal/iomcp` | MCP stdio helper exposing io worker tools. |
| `internal/persona` | Process controller for the selected harness. |
| `internal/personastate` | `~/.io` paths and persisted state. |
| `internal/soul` | First-run persona presets and `SOUL.md` rendering. |
| `internal/tui` | Bubble Tea model, chat UI, controls, dialogs, and styling. |
| `internal/workers` | Worker manager, lifecycle status, and harness runners. |
| `docs/superpowers` | Design specs and implementation plans. |
| `index.html` | Static visual prototype/mockup. |
