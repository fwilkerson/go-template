package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// gencheck hashes the files matching the git pathspecs, runs `just gen`, and
// fails when any of them changed. The regenerated files are left in place.
func gencheck(ctx context.Context, patterns []string, stdout io.Writer) error {
	if len(patterns) == 0 {
		return errors.New("gencheck: at least one pathspec is required")
	}
	files, err := trackedFiles(ctx, patterns)
	if err != nil {
		return err
	}
	before, err := digests(files)
	if err != nil {
		return err
	}
	gen := exec.CommandContext(ctx, "just", "gen")
	gen.Stderr = os.Stderr
	if err := gen.Run(); err != nil {
		return fmt.Errorf("just gen: %w", err)
	}
	after, err := digests(files)
	if err != nil {
		return err
	}
	if changed := changedFiles(before, after); len(changed) > 0 {
		return fmt.Errorf("generated files were behind their sources; just gen has updated them:\n  %s",
			strings.Join(changed, "\n  "))
	}
	_, err = fmt.Fprintf(stdout, "%d generated files are current\n", len(files))
	return err
}

// trackedFiles lists tracked and untracked-but-not-ignored files matching the
// pathspecs, so a freshly generated file counts as well.
func trackedFiles(ctx context.Context, patterns []string) ([]string, error) {
	args := append([]string{"ls-files", "--cached", "--others", "--exclude-standard", "--"}, patterns...)
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	return strings.Fields(string(out)), nil
}

func digests(files []string) (map[string][]byte, error) {
	sums := make(map[string][]byte, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		sums[f] = sum[:]
	}
	return sums, nil
}

// changedFiles returns, sorted, the paths whose digest differs between the two
// snapshots.
func changedFiles(before, after map[string][]byte) []string {
	var changed []string
	for f, sum := range after {
		if !bytes.Equal(before[f], sum) {
			changed = append(changed, f)
		}
	}
	slices.Sort(changed)
	return changed
}
