// Package arch holds the test that enforces the repository layout: which
// kinds of package may import which, forbidden package names, and the
// standard library over the modules it replaced. The rules are here; the
// engine and the layout it checks are documented in archtest.
package arch_test

import (
	"testing"

	"github.com/fwilkerson/plumb/archtest"
)

func TestLayout(t *testing.T) {
	t.Parallel()

	archtest.Test(t, archtest.Rules{
		Shell:          "internal/web",
		Tooling:        "internal/dev",
		ForbiddenNames: []string{"pkg", "utils", "util", "common", "models", "helpers"},
		// Package names the standard library has taken over from third-party
		// modules since the go directive in go.mod.
		StdlibNames: []string{"uuid", "errors", "slices", "maps"},
		// Development dependencies: only tooling imports them.
		ToolOnly: []string{"github.com/fwilkerson/plumb"},
	})
}
