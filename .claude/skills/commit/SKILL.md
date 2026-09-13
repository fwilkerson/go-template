---
name: commit
description: Create git commits in this repository's style - atomic Conventional Commits and a clean history with no rework commits. Use whenever committing, amending, squashing, or tidying history before a push.
---

# Commits

History records finished changes, not how they were reached. A reader of `git log` should see one commit per logical
change, each complete on its own. The failed attempts, lint fixes and rework from a session are not useful to them.

## Shape

- One logical change per commit. Split unrelated changes even when they were made together. A refactor that enables a
  feature is its own commit, before the feature.
- Stage by path (`git add <paths>`, or `git add -p` for part of a file), so the diff holds only the change being
  described.

## Rework is not history

Corrections to work that has not been pushed go into the commit they correct:

- It corrects the latest commit: `git commit --amend --no-edit`. Rewrite the message too if it no longer describes the
  change.
- It corrects an earlier unpushed commit: make a fixup commit, then squash it in without an interactive editor:
  ```sh
  git commit --fixup=<sha>
  GIT_SEQUENCE_EDITOR=: git rebase -i --autosquash <sha>~1
  ```

`fix:` is for a defect that already reached pushed history, not for correcting this session's own unpushed work.

Rewriting pushed history needs the user's go-ahead. When they agree, push with `--force-with-lease`, which fails if
someone else has pushed in the meantime.

## Message

Conventional Commits:

```
type(scope): subject

Body.

Trailers
```

- **type:** `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `build`, `ci`, `chore`, `style` or `revert`.
- **scope:** optional. The package or area touched, e.g. `auth`, `storage`, `claude`. Omit it for repo-wide changes.
- **subject:** imperative, lowercase, no trailing period, at most 72 characters. Say what the change does ("stop
  retrying a rejected upload"), not which files it touches.
- **breaking changes:** `type(scope)!: subject` plus a `BREAKING CHANGE:` footer.
- **body:** prose wrapped at 72 columns, not a list of files. Start with the problem or motivation: what was wrong or
  missing, with concrete evidence when there is some. Then say what the change does and any decision worth keeping.
  Leave the body out only when the subject says everything.
- **trailers:** last, after a blank line.

## Before each commit

1. Check whether this corrects unpushed work: `git log @{u}..`, or the whole log when there is no upstream. If it does,
   amend or fix up instead.
2. `git diff --staged` shows exactly one logical change.
