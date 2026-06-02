// Command io is a personal AI assistant TUI backed by a selected agent harness.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "io:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "mcp-stdio" {
		return runMCPStdio(os.Args[2:])
	}
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		return err
	}
	root, err := personastate.DefaultRoot()
	if err != nil {
		return err
	}
	st, err := personastate.Load(root)
	if err != nil {
		return err
	}
	if err := applyFlagOverrides(st, opts); err != nil {
		return err
	}

	a, err := newApp(root, *st, runtimeConfig{
		ClaudePath: opts.claudePath,
		CodexPath:  opts.codexPath,
	})
	if err != nil {
		return err
	}
	defer a.Close()

	model := tui.New(a)
	prog := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	a.attach(prog)

	_, err = prog.Run()
	return err
}

type cliOptions struct {
	harness    string
	model      string
	effort     string
	claudePath string
	codexPath  string
}

func parseFlags(args []string) (cliOptions, error) {
	var opts cliOptions
	fs := flag.NewFlagSet("io", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.harness, "harness", "", "agent harness to drive: claude or codex")
	fs.StringVar(&opts.harness, "agent-harness", "", "agent harness to drive: claude or codex")
	fs.StringVar(&opts.model, "model", "", "model override for the selected harness")
	fs.StringVar(&opts.model, "agent-model", "", "model override for the selected harness")
	fs.StringVar(&opts.effort, "effort", "", "reasoning effort: low, medium, or high")
	fs.StringVar(&opts.effort, "agent-effort", "", "reasoning effort: low, medium, or high")
	fs.StringVar(&opts.claudePath, "claude-path", "", "path to the claude binary")
	fs.StringVar(&opts.codexPath, "codex-path", "", "path to the codex binary")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	return opts, nil
}

func applyFlagOverrides(st *personastate.State, opts cliOptions) error {
	if opts.harness != "" {
		h, ok := agentharness.NormalizeKind(opts.harness)
		if !ok {
			return fmt.Errorf("unknown harness %q (want claude or codex)", opts.harness)
		}
		st.SetHarness(string(h))
	}
	if opts.model != "" {
		st.SetActiveModel(opts.model)
	}
	if opts.effort != "" {
		normalized := agentharness.NormalizeReasoningEffort(opts.effort)
		if normalized != strings.ToLower(strings.TrimSpace(opts.effort)) {
			return fmt.Errorf("unknown effort %q (want low, medium, or high)", opts.effort)
		}
		st.ReasoningEffort = normalized
		st.Normalize()
	}
	return nil
}
