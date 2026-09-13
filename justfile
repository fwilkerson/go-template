# Recipes are the shared interface for you, git hooks and Claude Code.

# List recipes
default:
    @just --list

# Format Go and Markdown
fmt:
    golangci-lint fmt
    dprint fmt

# Lint Go with golangci-lint and check formatting drift in Go and Markdown
lint:
    golangci-lint run
    dprint check

# Run tests; extra args pass through, e.g. `just test -run TestName`
test *args:
    go test ./... {{ args }}

# Apply go fix modernizers (run after bumping the go directive); `-diff` only reports
fix *args:
    go fix {{ args }} ./...

# Everything that must pass before finishing or committing
check: lint (fix "-diff") test
