// Package guidance holds the test that keeps the instructions written for
// Claude small and plainly worded. The budgets are here; the engine is
// guidancetest. When a budget is hit, move something into a tool, a test or
// a skill rather than raising it.
package guidance_test

import (
	"testing"

	"github.com/fwilkerson/plumb/guidancetest"
)

func TestGuidance(t *testing.T) {
	t.Parallel()

	guidancetest.Test(t, guidancetest.Config{
		AlwaysLoadedBudget: 800,
		SkillBudget:        800,
	})
}
