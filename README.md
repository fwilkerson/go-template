# go-template

Tooling and agent setup for a solo Go project worked on with GoLand and Claude Code.

## What's in it

| File                               | Purpose                                                                                                                                                                                                                                          |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `justfile`                         | The one set of task commands used by you, lefthook and Claude Code; `just gen` runs templ and Tailwind                                                                                                                                           |
| `.golangci.yml`                    | Correctness linters (`standard` + `errorlint`, `bodyclose`, `nilerr`) and formatters (`gofumpt`, `goimports`)                                                                                                                                    |
| `dprint.json`                      | Markdown formatting: 120-column lines, always wrapped                                                                                                                                                                                            |
| `.gitattributes`                   | Marks generated and vendored files so GitHub collapses their diffs and skips them in language stats                                                                                                                                              |
| `lefthook.yml`                     | Pre-commit: format staged Go, templ and Markdown files, then `just check`                                                                                                                                                                        |
| `cmd/server`, `internal/`          | Example app: thin `main` with its `.http` checks beside it, the `internal/greet` feature with its page, fragment and logic, the `internal/web` shell                                                                                             |
| `internal/web/static/`             | Vendored htmx and Alpine with their versions in `vendor.json`, plus the Tailwind build from `tailwind.css`                                                                                                                                       |
| `internal/dev/arch`                | The layout rules: which kinds of package may import which, forbidden package names, the standard library over the modules it replaced, and which modules only tooling may import. `plumb/archtest` enforces them                                 |
| `internal/dev/guidance`            | The guidance budgets: words Claude reads every turn and per skill. `plumb/guidancetest` enforces them, checks skill frontmatter and refuses shouting                                                                                             |
| `internal/dev`                     | Management tool behind `just gen-check`, `just vendor` and `just e2e`; the commands come from `plumb/devtool` and a project adds its own here. Everything under it is repository tooling: the arch test keeps application code from importing it |
| `.claude/settings.json`            | Allows `just` and `go doc`; denies the bare tools `just` wraps, `curl` and the git commands the commit skill steers away from; registers the Stop hook                                                                                           |
| `.claude/hooks/stop-check.sh`      | When a source `just check` covers changed, runs `just fmt check` before Claude finishes and sends failures back to it                                                                                                                            |
| `.claude/skills/commit/SKILL.md`   | Commit conventions: atomic Conventional Commits, no rework commits in history                                                                                                                                                                    |
| `.claude/skills/guidance/SKILL.md` | How to write instructions for Claude: prefer a lint, test, hook or recipe; where prose goes and how it reads                                                                                                                                     |
| `.claude/template.md`              | Go and task guidance for Claude, imported from `CLAUDE.md`                                                                                                                                                                                       |
| `CLAUDE.md`                        | Project notes; keep project-specific content here, not in `template.md`                                                                                                                                                                          |
| `github.com/fwilkerson/plumb`      | The tooling behind `internal/dev`: the `.http` runner, vendored asset updater, generated file check and the arch and guidance engines. A development dependency; the arch test refuses it from application code                                  |

### Who checks what

- **Modernization:** `go fix` (in `just check`) and GoLand's native inspections. `modernize` is left out of
  golangci-lint so the two don't lag or double-report.
- **Correctness:** golangci-lint, run by `just check` and shown in GoLand. Tests run with `-race` there, and `go mod
  tidy -diff` fails on a stale `go.mod` or `go.sum`.
- **Formatting:** golangci-lint's formatters for Go, `templ fmt` for templates and dprint for Markdown, at Stop and
  pre-commit only, never after each edit.
- **Generated files:** `*_templ.go` and `static/app.css` are committed so `go build` works from a clean checkout. `just
  check` regenerates them and fails on drift.
- **HTTP behaviour:** `cmd/server/*.http` are JetBrains HTTP Client files, kept beside the binary they exercise. GoLand
  runs them from the gutter with the `dev` environment; `just e2e` (in `just check`) builds the server, starts it on a
  free port and runs them with the stdlib runner in `plumb/httpfile`. The runner finds the binary as the only package
  under `cmd/`, so renaming it needs no configuration. It accepts the request syntax and the `client.test` /
  `client.assert` subset listed in its `assert.go`, and fails on a request without assertions or on anything outside the
  subset, so a file that passes here means the same in the IDE. Claude is denied `curl` against localhost; a
  verification request goes in a `.http` file instead, where it keeps running.
- **Vendored assets:** `just vendor` resolves each entry in `vendor.json` against its source and `-update` installs
  newer releases. GitHub sources take the file from the repository at the latest release tag, verified by git blob hash,
  and record that hash. npm sources exist for projects that commit no built file (Alpine); they download the package
  tarball and verify the registry's sha512. The report marks npm-sourced assets. A release younger than seven days is
  reported but not installed; `-force` overrides and `-cooldown` changes the age. Unauthenticated GitHub API calls are
  limited to 60 an hour; `GITHUB_TOKEN` lifts that.
- **Agent guidance:** the `guidance` skill loads when Claude is asked to write instructions for itself. It sends a rule
  to a lint, a test, a permission, a hook or a recipe first, and says how to write the rest. `internal/dev/guidance`
  keeps CLAUDE.md and its imports under a word budget, checks each skill's frontmatter and refuses shouting.
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
4. Install the git hooks with `lefthook install`. If git's `core.hooksPath` is set anywhere, lefthook refuses to sync;
   `lefthook install --reset-hooks-path` clears the setting and installs.
