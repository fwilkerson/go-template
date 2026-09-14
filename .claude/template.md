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

## Layout

- One `main` package per binary under `cmd/<name>/`. `main` parses arguments, wires dependencies and calls a `run(ctx,
  args, stdout) error` function; logic lives in packages.
- Everything else under `internal/`, one package per feature, named for what it provides, holding that feature's types,
  logic, storage, handlers and templates together. No `pkg/`, `utils`, `common` or `models` packages: a type two
  features share moves to a small package named for the concept, and interfaces are declared by the package that
  consumes them.
- `internal/web` is the shell: page layout, static assets and middleware. Features import it and expose a `Routes(mux
  *http.ServeMux)` function; `main` mounts them. The shell never imports a feature.
- Move a package out of `internal/` to a top-level directory only when another module imports it.
- Tests sit beside the code they test, in an external `_test` package unless they need unexported access. Fixtures go in
  `testdata/`.
- Web UI is templ pages and htmx fragments styled with Tailwind, with Alpine for client-only state. `*_templ.go` and
  `static/app.css` are generated: edit the `.templ` or `tailwind.css` source and run `just gen`. htmx and Alpine are
  vendored under `static/`.

## Tasks

Run every build, test, lint and format step through `just`: the recipes carry the flags and ordering the project needs,
and `just --list` describes every recipe.

- A recipe is a list of commands. Anything that needs a variable, a condition or a loop is a subcommand of the
  management tool in `internal/cmd/dev`, run as `just dev <command>`.
- `just check` passes before you finish.

## GoLand

When GoLand's MCP tools are available, call `lint_files` on the Go files you edited before finishing. It reports
GoLand's inspections, including golangci-lint results.
