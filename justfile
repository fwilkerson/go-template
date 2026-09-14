# Recipes are the shared interface for you, git hooks and Claude Code.

# List recipes
default:
    @just --list

# Regenerate templ output and the Tailwind stylesheet
gen:
    go generate ./...

# Format Go, templ and Markdown
fmt:
    golangci-lint fmt
    templ fmt .
    dprint fmt

# Lint Go with golangci-lint and check formatting drift in Go, templ and Markdown
lint:
    golangci-lint run
    templ fmt -fail .
    dprint check

# Run tests; extra args pass through, e.g. `just test -run TestName`
test *args:
    go test ./... {{ args }}

# Tidy go.mod and go.sum; `-diff` only reports
tidy *args:
    go mod tidy {{ args }}

# Apply go fix modernizers (run after bumping the go directive); `-diff` only reports
fix *args:
    go fix {{ args }} ./...

# Run the repository's management tool, e.g. `just dev vendor -update`
dev *args:
    go run ./internal/cmd/dev {{ args }}

# Fail when generated files are behind their sources; leaves them regenerated
gen-check:
    @just dev gencheck '*_templ.go' '**/static/app.css'

# Report vendored front-end assets with a newer release; `-update` installs them
vendor *args:
    @just dev vendor {{ args }}

# Run the .http checks beside the binary under cmd/ against a fresh build; `-addr host:port` targets a running one
e2e *args:
    @just dev e2e {{ args }}

# Everything that must pass before finishing or committing
check: lint (fix "-diff") (tidy "-diff") gen-check (test "-race") e2e
