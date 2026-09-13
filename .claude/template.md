# Go project conventions (from go-template)

## Go

- The target Go version is the `go` directive in `go.mod`. Use language and standard library features up to and
  including that version.
- Verify APIs with `go doc <pkg> <symbol>` rather than from memory, especially anything added in recent Go releases.
- `go fix` enforces most modern idioms. It cannot enforce these, so prefer them when writing new code:
  - `encoding/json/v2` for new JSON code (Go 1.27+); leave existing `encoding/json` code alone unless asked to migrate.
  - The standard library `uuid` package instead of third-party UUID modules (Go 1.27+).
  - Method and wildcard patterns on `http.ServeMux` with `r.PathValue` instead of a third-party router (Go 1.22+).
  - `cmp.Or` for fallback chains; `slices` and `maps` helpers instead of hand-written loops.
  - `errors.Join`, `context.WithCancelCause` and `context.Cause` when errors or cancellation reasons need to be combined
    or inspected.

## Tasks

Run every build, test, lint and format step through `just`: the recipes carry the flags and ordering the project needs,
and `just --list` describes every recipe.

- `just check` passes before you finish.

## GoLand

When GoLand's MCP tools are available, call `lint_files` on the Go files you edited before finishing. It reports
GoLand's inspections, including golangci-lint results.
