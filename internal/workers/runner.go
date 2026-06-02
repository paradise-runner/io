package workers

import (
	"context"

	"github.com/edward-champion/io/internal/agentharness"
)

// Runner executes a single worker request.
type Runner interface {
	Run(ctx context.Context, req Request) (Result, error)
}

// RunnerFunc adapts a function into a Runner.
type RunnerFunc func(context.Context, Request) (Result, error)

// Run implements Runner.
func (f RunnerFunc) Run(ctx context.Context, req Request) (Result, error) {
	return f(ctx, req)
}

// CommandRunner executes a harness CLI and returns stdout.
type CommandRunner = agentharness.CommandRunner
