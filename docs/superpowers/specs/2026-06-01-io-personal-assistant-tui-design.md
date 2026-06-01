# io — Personal AI Assistant TUI

**Status:** Design approved, pre-implementation
**Date:** 2026-06-01
**Author:** edward.champion (with Claude)

## Summary

`io` is a single, long-running personal AI assistant fronted by a chat-style TUI
(Bubble Tea, Go). It does **not** talk to a model API directly — Claude Code is its
engine throughout. `io` runs a persistent "persona" Claude session you chat with,
spawns ephemeral `claude -p` workers to do real work in other directories, and runs a
daily background "dreamer" that consolidates the day's conversations into durable
memory.

`io` is deliberately streamlined compared to multi-gateway/multi-provider assistants
(e.g. openclaw, hermes-agent): one surface (the TUI), one provider (Claude Code). Its
value over running `claude` directly is a **persistent assistant identity with its own
memory and personality**, an **orchestration layer** over many Claude Code sessions, and
**aggressive, automatic memory consolidation ("dreaming")** above what Claude Code does
on its own.

## Goals

- A persistent persona ("io") with a configurable personality that you chat with in an
  always-up TUI, decoupled from any single project directory.
- Persistent, searchable memory that survives restarts and surfaces in later sessions.
- A daily "dreaming" pass that harvests durable insights from conversations — distinct
  from context compaction.
- io-managed, TUI-configurable compaction of the persona session.
- Delegation: the persona spawns Claude Code workers in target directories, tracked by a
  glanceable spinner header, with results summarized back into the single chat thread.

## Non-Goals

- Multiple model providers. Claude Code is the only engine.
- Multiple front-end gateways (Slack, WhatsApp, web, voice). The TUI is the only surface.
- Wiring in external tools/services directly. Integrations come from skills/MCPs already
  available inside Claude Code, not from io.
- Rebuilding what Claude Code already provides (memory storage, context compaction,
  session persistence). io configures and orchestrates these; it does not reimplement
  them.

## Background & Research

### Reference architectures

- **openclaw / hermes-agent**: both are versatile personal assistants, but both drive
  models via SDK/HTTP and treat Claude Code as a *downstream tool*. io is the inverse:
  Claude Code is the **engine**. The directly transferable patterns: a long-lived daemon
  as control plane; a `SOUL.md` (static personality) + `MEMORY.md` (curated durable
  memory) split; a context compressor; SQLite/FTS-style searchable history; subagent
  spawning surfaced as TUI overlays/spinners.
- **claw-orchestrator** (the closest real reference to io's pattern): wraps Claude
  Code/Codex/etc. as *persistent, programmable headless sessions* rather than naive
  one-shot subprocesses.

### What Claude Code provides out of the box (reuse, do not rebuild)

- **Auto memory** (v2.1.59+): `~/.claude/projects/<proj>/memory/` with a `MEMORY.md`
  index + topic files; on by default; works in `-p`. **Relocatable** via the
  `autoMemoryDirectory` setting — so io points the persona at its own memory directory,
  decoupled from any project. Toggle via `autoMemoryEnabled` /
  `CLAUDE_CODE_DISABLE_AUTO_MEMORY`.
- **Compaction**: automatic auto-compact + manual `/compact [focus]` are built in.
- **Streaming**: `--output-format stream-json --input-format stream-json` keeps one
  persona process alive and accepts many turns over stdin. Events include `system/init`
  (carries `session_id`, model), `stream_event` (`text_delta`, tool_use), and a final
  `result` (with `total_cost_usd`, usage).
- **MCP in headless mode**: confirmed. io exposes *itself* as an MCP server (stdio) that
  the persona and workers connect to via `--mcp-config`.
- **Headless workers**: `claude -p` with `--output-format json`, `--json-schema`,
  `--add-dir`, `--permission-mode`, `--allowedTools`, `--max-turns`, `--model`,
  `--mcp-config`. Final `result` message + exit signals completion.
- **Transcripts**: JSONL at `~/.claude/projects/<cwd-encoded>/<session-id>.jsonl`;
  external programs can read them (input for dreaming). io reads, never writes them.

### What io must build

- **Dreaming** — there is **no** built-in background memory harvesting. (Anthropic
  "Claude Dreaming"/Auto Dream is a separate research-preview concept, not in Claude
  Code.) io builds this.
- **Forced session continuity** — there is no `--session-id` flag to set an ID. io
  captures the auto-generated persona `session_id` on first launch and `--resume`s it
  thereafter.
- The **Bubble Tea TUI**, the **persona process lifecycle**, the **io MCP server +
  worker orchestration + spinner header**, and the **compaction controller**.

### Dreaming vs. compaction (load-bearing distinction)

| | Compaction | Dreaming / consolidation |
|---|---|---|
| Goal | Shrink the live context window so a session continues | Generate & preserve insight across sessions |
| When | Reactive, near context threshold | Proactive, once a day when idle |
| Operation | High-fidelity summary replacing older turns | Reflect → extract → recombine → curate → store |
| Output | Smaller version of the same information | New, higher-level information |
| Scope | Current persona session | Persistent cross-session memory |

Dreaming mechanism is based on Stanford Generative Agents *reflection*
(arxiv 2304.03442), hardened with Claude-Dreaming-style curation
(dedup/merge/contradiction-resolution) and Letta sleep-time-agent structure (separate
background writer, stronger offline model).

## Architecture

`io` is a single Go binary: Bubble Tea TUI + an embedded MCP server in one process. It
owns three subsystems — the **persona**, the **workers**, and the **dreamer** — plus a
**compaction controller**.

```
┌─ io (Go binary: Bubble Tea TUI + embedded MCP server) ────────┐
│                                                                │
│  persona ── persistent `claude` process                        │
│    --output-format stream-json --input-format stream-json      │
│    --resume <persona-session-id>   (captured once, reused)     │
│    --append-system-prompt-file SOUL.md   (io's personality)    │
│    autoMemoryDirectory = ~/.io/memory   (io's own memory)      │
│    --mcp-config → io's MCP server  (spawn_worker, etc.)         │
│                                                                │
│  workers ── ephemeral `claude -p` processes                    │
│    spawned in target dirs, --output-format json --json-schema  │
│    tracked in spinner header, result injected back to persona  │
│                                                                │
│  dreamer ── daily `claude -p` consolidation pass               │
│    reads persona transcript JSONL + memory/, harvests insights │
│                                                                │
│  compaction controller ── watches persona usage, triggers      │
│    /compact at a TUI-configurable threshold                    │
└────────────────────────────────────────────────────────────────┘
```

### Components

Each component has one clear purpose and a well-defined interface, so it can be tested in
isolation against a fake `claude` binary.

#### TUI (Bubble Tea)
- **Does:** renders the single chat thread from streamed `text_delta` events; renders the
  spinner header for active workers; captures user input and hands turns to the persona;
  exposes settings (compaction threshold, manual "compact now", dream cadence).
- **Interface:** consumes a channel of persona/worker events from the orchestrator; emits
  user-turn and command messages.
- **Depends on:** orchestrator (events in, commands out). No direct knowledge of `claude`.

#### Persona controller
- **Does:** owns the single persistent `claude` streaming process that *is* "io". On
  first launch, starts it, captures `session_id` from `system/init`, persists it.
  Thereafter `--resume`s that id. Feeds user turns over stdin; parses stream-json events
  out. On process death, restarts and resumes; if resume fails, forks a fresh session and
  notes it in the thread.
- **Interface:** `SendTurn(text)`, event stream out (`text_delta`, `tool_use`, `result`,
  `usage`).
- **Depends on:** the `claude` binary; `~/.io/state.json`; `~/.io/SOUL.md`.

#### io MCP server (embedded)
- **Does:** exposes orchestration tools to the persona (and, where useful, workers):
  - `spawn_worker(cwd, task) -> {worker_id}` — async; returns immediately.
  - `worker_status() -> [{worker_id, dir, label, state}]`
  - `list_projects() -> [dirs]` — scans known project roots (e.g. `~/git`).
- **Interface:** standard MCP over stdio, wired into the persona via `--mcp-config`.
- **Depends on:** the worker manager.

#### Worker manager
- **Does:** launches `claude -p` in the target dir
  (`--output-format json --json-schema <result-schema> --max-turns <n>` + a timeout, each
  with its own session id), tracks lifecycle, updates the spinner header
  (`● cgw#1 ⏳` / `✓` / `✗`). On completion (or failure/timeout), injects a fresh turn into
  the persona stream summarizing the result so io surfaces it in the thread on its own.
- **Interface:** `Spawn(cwd, task) -> worker_id`, `Status()`, completion callback →
  persona controller.
- **Depends on:** the `claude` binary; persona controller (for result injection).

#### Compaction controller
- **Does:** watches the persona's `usage` from stream-json; when context crosses a
  TUI-configurable threshold (default ~75% of window), triggers `/compact` on the persona
  session. Also handles the manual "compact now" command. Never touches memory/dreaming.
- **Interface:** consumes usage events; calls `persona.SendCommand("/compact")`.
- **Depends on:** persona controller; settings.

#### Dreamer
- **Does:** the daily consolidation pass (mechanism below). Triggered by a scheduler.
- **Interface:** `MaybeDream(now)` — checks cadence/threshold, runs a `claude -p` pass.
- **Depends on:** persona transcript JSONL (read-only); `~/.io/memory/`; settings;
  `~/.io/state.json` (last-dream timestamp, chat counter).

## Onboarding & Personality

First run walks the user through a short setup that writes `~/.io/SOUL.md`. The user
picks a **base persona** (all aimed at people who want a capable personal assistant):

- **The Staff Engineer** — terse, technical, opinionated; assumes deep expertise, skips
  hand-holding, leads with the answer.
- **The Chief of Staff** — proactive and organizing; tracks threads, nudges follow-ups;
  good for busy / heavily-scheduled people (e.g. busy parents) who want things *managed*.
- **The Pair Partner** — collaborative and curious; thinks out loud, asks before acting;
  good for exploratory work.
- **Blank slate** — minimal persona the user writes themselves.

Two orthogonal knobs layer on top and compose into the generated `SOUL.md`:

- **Verbosity** — terse ↔ thorough.
- **Proactivity** — reactive ↔ anticipates.

`SOUL.md` is passed to the persona via `--append-system-prompt-file` on every launch
(append-system-prompt does not survive `--resume`). It is hand-editable at any time.

## The Dreamer (detailed)

Standalone daily subsystem, independent of compaction.

**Trigger:** at most once per day, and only if there have been **≥ 7 chats** since the
last dream (configurable). A "chat" is defined as one user-turn → io-response exchange.
io maintains a per-day chat counter and a last-dream timestamp in `~/.io/state.json`.

**Pass** (a `claude -p` job, configurably on a stronger model since nobody is waiting):

1. **Gather** the day's persona transcript turns (episodic memory; Claude already stores
   these as JSONL).
2. **Reflect** — generate the most salient high-level questions, then answer them →
   candidate insights, each shaped as
   `{insight, evidence_pointers[], confidence, tags[]}`.
3. **Recombine** — connect items across different days/topics into novel hypotheses,
   tagged speculative (this is what makes it *dreaming*, not summarizing).
4. **Curate on write** — dedup/merge/contradiction-check each candidate against existing
   memory: WRITE / MERGE / UPDATE / SKIP.
5. **Store** durables into `~/.io/memory/` (the `autoMemoryDirectory` the persona reads),
   so dreams surface naturally in the next session.

Episodic memory (raw transcripts) grows linearly and decays via Claude Code's
`cleanupPeriodDays`; semantic memory (insights) grows slowly and persists.

## Compaction (detailed)

Separate and reactive. io watches the persona's context usage from stream-json `usage`
data and triggers `/compact` on the persona session when it crosses a **configurable
threshold** (default ~75% of window), exposed in the TUI alongside a manual "compact now"
control. Stays entirely within the single session io manages; never touches memory or
dreaming.

## Persistence & State

- `~/.io/SOUL.md` — personality (generated at onboarding, hand-editable).
- `~/.io/state.json` — persona `session_id`, last-dream timestamp, per-day chat counter,
  known project dirs, settings (compaction threshold, dream cadence/threshold, dreamer
  model).
- `~/.io/memory/` — the persona's `autoMemoryDirectory` (Claude-managed +
  dreamer-augmented).
- Transcripts remain where Claude writes them
  (`~/.claude/projects/.../<session-id>.jsonl`); io reads, never writes.

## Conversation & Delegation Model

- **Single thread.** You always talk to "io" in one conversation. Workers are plumbing.
- **Glanceable header.** A spinner row shows distinct active workers
  (`● cgw#1 ⏳  ● deep#2 ✓`).
- **Delegation via io's MCP tool**, not Claude's native Task tool, so io has full
  lifecycle visibility for the header.
- **Async spawning.** `spawn_worker` returns a `worker_id` immediately; you keep chatting;
  on completion io injects the result as a fresh persona turn.

## Error Handling & Edge Cases

- **Persona process dies:** io restarts and `--resume`s the stored id; if resume fails,
  fork a fresh session and note it in the thread.
- **Worker crash/timeout:** bounded by `--max-turns` + a wall-clock timeout; spinner shows
  `✗`; a failure summary is injected to the persona.
- **Concurrency:** workers are independent processes with unique session ids (no transcript
  collision). Auth is global; Anthropic rate limits are the only real ceiling.
- **Dream while chatting:** the dreamer is a separate `claude -p` process reading
  read-only transcripts; it does not block the persona.
- **First run / no session yet:** onboarding completes before the persona session is
  created; `session_id` captured on the first real turn.

## Testing Strategy

- **Fake `claude` binary:** a test double that emits canned stream-json / JSON results,
  letting orchestrator logic (worker lifecycle, session-id capture/resume, MCP tool
  handlers, compaction triggering, dream cadence gating) be unit-tested fast and offline.
- **Go unit tests** for each component against that double.
- **One integration smoke test** that runs the real `claude -p` once to validate flag
  wiring and event parsing.
- **Dreamer tests:** feed a fixture transcript through the pass and assert structured
  insights + correct WRITE/MERGE/SKIP curation decisions against a seeded memory dir.

## v1 Scope

All four pillars are in v1:

1. Chat with persona in an always-up TUI (persona controller + TUI + onboarding/SOUL.md).
2. Persistent + searchable memory (reuse Claude Code auto memory via
   `autoMemoryDirectory`).
3. Dreaming / consolidation (the custom daily pass).
4. Worker spawning + spinner header (io MCP server + worker manager).

Plus the compaction controller (small, and required to keep the always-up persona
healthy).

## Open Questions / To Confirm

- **"Chat" definition:** currently one user-turn → io-response exchange. Confirm this is
  the intended unit for the ≥7/day dream threshold (vs. distinct conversation sessions).
- **Persona base personas:** confirm the four (Staff Engineer / Chief of Staff / Pair
  Partner / Blank slate) are the right starting set.
- **Worker transcripts in dreaming:** v1 dreams only over the persona transcript; whether
  to also fold in worker transcripts is deferred.

## Future / Out of v1

- Switchable/drill-in worker views (watch or steer a worker live).
- Multi-pane "mission control" dashboard.
- Counterfactual/verification gating for speculative dream insights.
- Importance-weighted dream triggering (vs. simple chat-count threshold).
