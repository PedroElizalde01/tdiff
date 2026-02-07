# TDiff (Terminal Diff)

TDiff is a read-only terminal UI for Git diffs with a VS Code-like side-by-side view and fast keyboard navigation.

## Run

```bash
go run .
```

Run it inside a Git repository.

## What It Does

- Shows changed files in a left `CHANGES` sidebar.
- Shows side-by-side diff panes for the selected file:
  - left pane: `OLD`
  - right pane: `NEW`
- Supports both worktree and staged changes.
- Uses `git` shell commands directly (no libgit2).

## Keybindings

- Global:
  - `q` or `Ctrl+C`: quit
  - `s`: toggle mode (`WORKTREE` / `STAGED`)
- Files sidebar focus:
  - `Up`/`Down` or `k`/`j`: move file selection
  - `Enter` or `Right`: move focus to `OLD` pane
- Old pane focus:
  - `Up`/`Down` or `k`/`j`: move cursor-line
  - `Left`: move focus to files sidebar
  - `Right`: move focus to `NEW` pane
- New pane focus:
  - `Up`/`Down` or `k`/`j`: move cursor-line
  - `Left`: move focus to `OLD` pane
  - `Right`: no-op
- Diff navigation:
  - `n`: next hunk header
  - `p`: previous hunk header
  - `g`: top of diff
  - `G`: bottom of diff

## Notes

- If there are no changes, TDiff shows `(no changes)` and `(no diff)`.
- If the current repo is invalid / not a Git repo, TDiff shows a friendly error in the UI.
- Binary diffs are rendered as `(binary file changed)`.

