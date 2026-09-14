// Command dev is the repository's management tool. justfile recipes call it
// for anything that needs more than a single command.
//
// Usage:
//
//	dev gencheck <pattern>...   fail when generated files are behind their sources
//	dev vendor [-update] [-force]  report or refresh the vendored front-end assets
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "dev:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New("usage: dev <gencheck|vendor> [args]")

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}
	switch args[0] {
	case "gencheck":
		return gencheck(ctx, args[1:], stdout)
	case "vendor":
		return vendor(ctx, args[1:], stdout, stderr)
	}
	return fmt.Errorf("unknown command %q: %w", args[0], errUsage)
}
