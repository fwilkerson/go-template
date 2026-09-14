---
name: guidance
description: Write or change instructions meant for Claude - CLAUDE.md, template.md, a SKILL.md, a hook or agent prompt, or a "remember to" note. Use before adding any rule, convention or reminder for an agent, and when asked to make an agent behave differently.
---

# Guidance

CLAUDE.md and its imports are read on every turn of every session, competing with the task for attention. A rule a tool
enforces costs no attention and cannot be skipped. So the first question is not how to phrase a rule but whether it
needs to be prose at all.

## Reach for a mechanism first

Take the first row that fits:

| The rule is about                        | Put it in                                             |
| ---------------------------------------- | ----------------------------------------------------- |
| The shape of code                        | `.golangci.yml`, `dprint.json`, `go fix`              |
| An invariant of the repository           | A test, like `internal/arch` or `internal/guidance`   |
| A command that may or may not be run     | `permissions` in `.claude/settings.json`              |
| Something that happens at a fixed moment | A hook in `settings.json` or `lefthook.yml`           |
| A sequence run the same way every time   | A `just` recipe, with any logic in `internal/cmd/dev` |
| A judgment call: what to prefer, and why | Prose, as below                                       |

A mechanism replaces prose: delete the sentence it makes redundant.

## When it has to be prose

- **Put it where it is read.** CLAUDE.md and template.md hold only what every task needs. A skill loads when its
  description matches the task, so task-specific procedure goes there, with a description in the words a task would use.
  `internal/guidance` fails when the always-loaded set outgrows its budget; move something out rather than raising it.
- **Say what to do and why.** One reason lets the model generalize; a bare rule is followed literally and only where it
  was written. "Verify APIs with `go doc`, since recent additions are not in memory" beats "use `go doc`".
- **Describe the situation, not the emphasis.** Current models over-trigger on capitals, bold commands and repetition.
  State when the rule applies, once, in a normal register. `internal/guidance` refuses shouting.
- **Show the shape when the shape matters.** One realistic example in `<example>` tags, with a sentence on why it is
  right, steers format and tone better than a paragraph describing it.
- **Prefer the goal to the step list.** Numbered steps only where order or completeness matters; otherwise give the goal
  and the constraints and leave the method to the model.
- **Positive form.** "Stage by path" over "don't use git add -A". A prohibition needs its reason attached.
- **Run the colleague test.** A capable colleague with no context on this repo reads it. If they would ask a question,
  the model will guess.

## Sources

The rules above are the repo-sized version of Anthropic's guidance. Read the originals when writing a longer prompt,
such as a subagent brief:

- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices.md
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1.md
