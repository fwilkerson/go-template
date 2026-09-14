// Command dev is the repository's management tool. justfile recipes call it
// for anything that needs more than a single command.
//
// Usage:
//
//	dev gencheck <pattern>...   fail when generated files are behind their sources
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

var errUsage = errors.New("usage: dev gencheck [args]")

func run(ctx context.Context, args []string, stdout, _ io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}
	switch args[0] {
	case "gencheck":
		return gencheck(ctx, args[1:], stdout)
	}
	return fmt.Errorf("unknown command %q: %w", args[0], errUsage)
}
