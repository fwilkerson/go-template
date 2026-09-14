package main

import (
	"bytes"
	"context"
	"testing"
)

func TestRunRejectsUnknownFlag(t *testing.T) {
	t.Parallel()

	if err := run(context.Background(), []string{"-nope"}, &bytes.Buffer{}); err == nil {
		t.Fatal("run: expected an error for an unknown flag")
	}
}

func TestRunStopsWhenContextEnds(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, []string{"-addr", "localhost:0"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
}
