package greet_test

import (
	"testing"

	"github.com/fwilkerson/go-template/internal/greet"
)

func TestGreeting(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name string
		want string
	}{
		"named": {name: "Go", want: "Hello, Go!"},
		"empty": {name: "", want: "Hello, world!"},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			if got := greet.Greeting(tc.name); got != tc.want {
				t.Errorf("Greeting(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}
