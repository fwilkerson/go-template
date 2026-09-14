# Go project conventions (from go-template)

## Go

- The target Go version is the `go` directive in `go.mod`. Use language and standard library features up to and
  including that version.
- Verify APIs with `go doc <pkg> <symbol>` rather than from memory, especially anything added in recent Go releases.
- `go fix` enforces most modern idioms. It cannot enforce these, so prefer them when writing new code:
  - `encoding/json/v2` for new JSON code (Go 1.27+); leave existing `encoding/json` code alone unless asked to migrate.
  - Method and wildcard patterns on `http.ServeMux` with `r.PathValue` (Go 1.22+) and the standard library `uuid`
    package (Go 1.27+). `internal/arch` requires `Routes` to take a `*http.ServeMux` and refuses third-party packages
    named for one the standard library now has.
  - `cmp.Or` for fallback chains; `slices` and `maps` helpers instead of hand-written loops.
  - `errors.Join`, `context.WithCancelCause` and `context.Cause` when errors or cancellation reasons need to be combined
    or inspected.

## Layout

- Binaries under `cmd/<name>/`: a thin `main` that calls `run(ctx, args, stdout) error`. Logic lives in packages.
- Everything else under `internal/`, one package per feature holding its types, logic, storage, handlers and templates.
  A feature exposes `Routes(mux *http.ServeMux)` and `main` mounts it. `internal/web` is the shell: layout, static
  assets and middleware. A type two features need moves to a package named for the concept.
- `internal/arch` is the test that decides which kinds of package may import which, which package names are refused and
  where the standard library must be used over a third-party module. Read it before adding a package or dependency;
  extend it when the layout gains a rule.
- Tests beside the code in an external `_test` package unless they need unexported access; fixtures in `testdata/`.
- Checks over HTTP are `.http` requests with assertions beside the binary in `cmd/<name>/`, run by `just e2e` and by
  GoLand. Add a request there instead of calling the server by hand: it is the record of what was verified and keeps
  running after you.
- Web UI is templ pages and htmx fragments styled with Tailwind, with Alpine for client-only state. `*_templ.go` and
  `static/app.css` are generated: edit the `.templ` or `tailwind.css` source and run `just gen`. htmx and Alpine are
  vendored under `static/` and listed in `vendor.json`; `just vendor` reports newer releases and `just vendor -update`
  installs them. Source new assets from GitHub releases rather than npm: the npm registry has been the vector for a run
  of supply chain attacks, so it is used only when a project commits no built file.

## Tasks

Run every build, test, lint and format step through `just`: the recipes carry the flags and ordering the project needs,
and `just --list` describes every recipe.

- A recipe is a list of commands. Anything that needs a variable, a condition or a loop is a subcommand of the
  management tool in `internal/cmd/dev`, run as `just dev <command>`.
- `just check` passes before you finish.

## GoLand

When GoLand's MCP tools are available, call `lint_files` on the Go files you edited before finishing. It reports
GoLand's inspections, including golangci-lint results.
