// Command dev is the repository's management tool. justfile recipes call it
// for anything that needs more than a single command. The standard commands
// come from plumb; a project adds its own beside them. Its subpackages hold
// the tests that keep the layout and the agent guidance to their rules; the
// arch test keeps application code from importing any of it.
//
// Usage:
//
//	dev gencheck <pattern>...              fail when generated files are behind their sources
//	dev vendor [-update] [-force]          report or refresh the vendored front-end assets
//	dev e2e [-addr host:port] [file...]    run the .http checks beside the binary under cmd/
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/fwilkerson/plumb/devtool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "dev:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cmds := devtool.Standard()
	// Project-specific commands go here: cmds["name"] = func(...) error { ... }
	return devtool.Run(ctx, args, stdout, stderr, cmds)
}
