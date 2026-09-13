#!/bin/sh
# Claude Code Stop hook: format and run `just check` before Claude finishes.
# Exit 2 sends the output back to Claude so it fixes the failures.
set -u

input=$(cat)

# Claude is already continuing because of this hook; let it stop rather than
# risk looping on a failure it cannot fix.
if printf '%s' "$input" | grep -Eq '"stop_hook_active"[[:space:]]*:[[:space:]]*true'; then
	exit 0
fi

root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$root" || exit 0

# Nothing to check unless a source `just check` covers changed in the working tree.
if [ -z "$(git status --porcelain -- '*.go' '*.templ' '*.css' '*.md' '*.http' go.mod go.sum)" ]; then
	exit 0
fi

# A missing tool is a machine setup problem Claude cannot fix; warn the user
# instead of blocking.
if ! command -v just >/dev/null 2>&1; then
	printf '{"systemMessage": "stop-check skipped: just is not installed"}\n'
	exit 0
fi

if ! output=$(just fmt check 2>&1); then
	printf '%s\n\n%s\n' "$output" "just check failed. Fix the issues above before finishing." >&2
	exit 2
fi
