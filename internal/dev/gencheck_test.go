package main

import (
	"slices"
	"testing"
)

func TestChangedFiles(t *testing.T) {
	t.Parallel()

	before := map[string][]byte{"a": {1}, "b": {2}}
	after := map[string][]byte{"a": {1}, "b": {3}, "c": {4}}

	got := changedFiles(before, after)
	want := []string{"b", "c"}
	if !slices.Equal(got, want) {
		t.Errorf("changedFiles = %v, want %v", got, want)
	}
	if got := changedFiles(before, before); len(got) != 0 {
		t.Errorf("changedFiles(same) = %v, want none", got)
	}
}
