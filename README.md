# go-template

Tooling and agent setup for a solo Go project worked on with GoLand and Claude Code.

## What's in it

| File                             | Purpose                                                                                                               |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `justfile`                       | The one set of task commands used by you, lefthook and Claude Code; `just gen` runs templ and Tailwind                |
| `.golangci.yml`                  | Correctness linters (`standard` + `errorlint`, `bodyclose`, `nilerr`) and formatters (`gofumpt`, `goimports`)         |
| `dprint.json`                    | Markdown formatting: 120-column lines, always wrapped                                                                 |
| `.gitattributes`                 | Marks generated and vendored files so GitHub collapses their diffs and skips them in language stats                   |
| `lefthook.yml`                   | Pre-commit: format staged Go, templ and Markdown files, then `just check`                                             |
| `cmd/server`, `internal/`        | Example app: thin `main`, the `internal/greet` feature with its page, fragment and logic, the `internal/web` shell    |
| `internal/web/static/`           | Vendored htmx and Alpine, plus the Tailwind build from `tailwind.css`                                                 |
| `.claude/settings.json`          | Allows `just` and `go doc`, denies the bare tools `just` wraps, turns off commit attribution, registers the Stop hook |
| `.claude/hooks/stop-check.sh`    | When Go files changed, runs `just fmt check` before Claude finishes and sends failures back to it                     |
| `.claude/skills/commit/SKILL.md` | Commit conventions: atomic Conventional Commits, no rework commits in history                                         |
| `.claude/template.md`            | Go and task guidance for Claude, imported from `CLAUDE.md`                                                            |
| `CLAUDE.md`                      | Project notes; keep project-specific content here, not in `template.md`                                               |

### Who checks what

- **Modernization:** `go fix` (in `just check`) and GoLand's native inspections. `modernize` is left out of
  golangci-lint so the two don't lag or double-report.
- **Correctness:** golangci-lint, run by `just check` and shown in GoLand.
- **Formatting:** golangci-lint's formatters for Go, `templ fmt` for templates and dprint for Markdown, at Stop and
  pre-commit only, never after each edit.
- **Generated files:** `*_templ.go` and `static/app.css` are committed so `go build` works from a clean checkout; `just
  gen` rebuilds them.
- **Agent feedback:** GoLand's MCP server (`lint_files`) while the IDE is open; the Stop hook otherwise.

## Machine setup (once)

1. Install `golangci-lint`, `just`, `lefthook`, `templ` and the standalone `tailwindcss` CLI (managed with `prov`), and
   `dprint`. The `templ` CLI version must match the `github.com/a-h/templ` version in `go.mod`.
2. GoLand: **Settings → Tools → Go Linter**: point it at the `golangci-lint` binary and enable using the project config
   file.
3. GoLand: **Settings → Tools → MCP Server → Clients Auto-Configuration**: Auto-Configure for Claude Code.

## Project setup

1. Copy everything except this README into the project.
2. Set the module path in `go.mod` and rename `cmd/server` to the binary's name. Keep `internal/web` for a web app and
   delete it otherwise; `internal/greet` is a placeholder for the first feature.
3. Put the project's name and notes in `CLAUDE.md`, keeping the `@.claude/template.md` import.
4. Install the git hooks. The global git config points `core.hooksPath` at an empty directory, so the repo sets its own
   path. Lefthook still sees the global setting and refuses, so `--force` is needed; it installs into the local path.
   ```sh
   git config --local core.hooksPath "$PWD/.git/hooks"
   lefthook install --force
   ```
