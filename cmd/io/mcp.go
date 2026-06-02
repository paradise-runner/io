package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/edward-champion/io/internal/ioipc"
	"github.com/edward-champion/io/internal/iomcp"
)

func runMCPStdio(args []string) error {
	var socket string
	fs := flag.NewFlagSet("io mcp-stdio", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&socket, "control-socket", "", "path to the parent io control socket")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments for mcp-stdio")
	}
	if socket == "" {
		return fmt.Errorf("--control-socket is required")
	}
	client := ioipc.Client{SocketPath: socket}
	return iomcp.New(client).Serve(context.Background(), os.Stdin, os.Stdout)
}
