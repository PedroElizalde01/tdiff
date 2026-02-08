<div align="center">
<pre>
     ████████╗██████╗ ██╗███████╗███████╗
     ╚══██╔══╝██╔══██╗██║██╔════╝██╔════╝
      ██║   ██║  ██║██║█████╗  █████╗
      ██║   ██║  ██║██║██╔══╝  ██╔══╝
   ██║   ██████╔╝██║██║     ██║
   ╚═╝   ╚═════╝ ╚═╝╚═╝     ╚═╝
</pre>

Read-only terminal diff viewer for Git
</div>

## Overview

TDiff is a Bubble Tea + Lipgloss TUI that shows Git diffs in a side-by-side layout:

- Left sidebar: changed files (`FILES CHANGED`)
- Main view: `OLD` and `NEW` panes
- Fast keyboard navigation (Yazi-like focus flow)
- Intra-line (word-level) highlighting for edit pairs

TDiff is read-only: it never stages, unstages, or writes Git state.

## Features

- Worktree and staged views (`s` to toggle)
- Diff algorithm cycling (`a`): `default` -> `histogram` -> `patience`
- Per-file status badges in sidebar:
  - `M` modified
  - `A` added
  - `D` deleted
  - `R` renamed/copied
  - `U` untracked
- Hunk navigation (`n` / `p`) and top/bottom jump (`g` / `G`)
- Cursor-line persistence per selected file
- Binary diff fallback: `(binary file changed)`
- Friendly error when outside a Git repository

## Requirements

- Go 1.18+
- Git available in `PATH`
- Run TDiff from inside a Git repository

## Run

```bash
go run .
```

## Build

```bash
go build -o tdiff .
./tdiff
```

## Keybindings

| Keys | Action |
|---|---|
| `q` / `Ctrl+C` | Quit |
| `s` | Toggle mode (`WORKTREE` / `STAGED`) |
| `a` | Cycle diff algorithm |
| `Up`/`Down` or `k`/`j` | Move cursor |
| `Left` / `Right` | Change focus |
| `n` / `p` | Next / previous hunk |
| `g` / `G` | Top / bottom |

## Diff Sources

TDiff shells out to Git (no libgit2):

- File lists:
  - `git diff --name-only`
  - `git diff --cached --name-only`
- Per-file diffs:
  - worktree/staged diff with `--no-color --unified=3`
  - untracked files via `--no-index /dev/null <file>`

Algorithm flags are applied when selected (`--histogram` / `--patience`) and fall back to default when unsupported.

## Notes

- If there are no changes, TDiff shows `(no changes)` and `(no diff)`.
- Unified diff header lines (`diff --git`, `index`, `---`, `+++`) are hidden in panes for cleaner code-focused reading.
- Some terminals may render box/border characters differently depending on font and locale.
