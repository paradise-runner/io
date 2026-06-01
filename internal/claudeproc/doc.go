// Package claudeproc parses the stream-json wire format emitted and consumed by
// the `claude` CLI in headless/streaming mode.
package claudeproc

// Version is the wire-contract version this package targets. Bumped when the
// fake claude binary and the real binary's format are reconciled.
const Version = "phase1"
