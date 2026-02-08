package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Mode int

const (
	Worktree Mode = iota
	Staged
)

// DiffAlgo controls which git diff algorithm flag is used when loading file diffs.
type DiffAlgo int

const (
	DiffDefault DiffAlgo = iota
	DiffHistogram
	DiffPatience
)

func (m Mode) String() string {
	if m == Staged {
		return "STAGED"
	}
	return "WORKTREE"
}

func (m Mode) Toggle() Mode {
	if m == Staged {
		return Worktree
	}
	return Staged
}

func (a DiffAlgo) String() string {
	switch a {
	case DiffHistogram:
		return "histogram"
	case DiffPatience:
		return "patience"
	default:
		return "default"
	}
}

func (a DiffAlgo) Next() DiffAlgo {
	switch a {
	case DiffDefault:
		return DiffHistogram
	case DiffHistogram:
		return DiffPatience
	default:
		return DiffDefault
	}
}

func ListChangedFiles(mode Mode) ([]string, error) {
	if mode == Staged {
		return listFilesStaged()
	}
	return listFilesWorktree()
}

func FileStatuses(mode Mode) (map[string]string, error) {
	if mode == Staged {
		return stagedStatuses()
	}
	return worktreeStatuses()
}

func listFilesWorktree() ([]string, error) {
	out, err := runGit("diff", "--name-only")
	if err != nil {
		return nil, err
	}

	files := parseNonEmptyLines(out)
	untrackedOut, err := runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return appendUnique(files, parseNonEmptyLines(untrackedOut)), nil
}

func listFilesStaged() ([]string, error) {
	out, err := runGit("diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	return parseNonEmptyLines(out), nil
}

func worktreeStatuses() (map[string]string, error) {
	out, err := runGit("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	statuses := map[string]string{}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for _, line := range lines {
		if len(line) < 3 || strings.TrimSpace(line) == "" {
			continue
		}

		code := line[:2]
		path := parsePorcelainPath(strings.TrimSpace(line[3:]))
		if path == "" {
			continue
		}

		status := normalizeStatusCode(code)
		if status == "" {
			continue
		}
		statuses[path] = status
	}

	// Ensure untracked files are always labeled consistently with the files list.
	untrackedOut, err := runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, path := range parseNonEmptyLines(untrackedOut) {
		statuses[path] = "?"
	}
	return statuses, nil
}

func stagedStatuses() (map[string]string, error) {
	out, err := runGit("diff", "--cached", "--name-status")
	if err != nil {
		return nil, err
	}

	statuses := map[string]string{}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		code := normalizeStatusCode(parts[0])
		if code == "" {
			continue
		}

		pathIdx := 1
		if strings.HasPrefix(parts[0], "R") || strings.HasPrefix(parts[0], "C") {
			pathIdx = len(parts) - 1
		}
		if pathIdx < 0 || pathIdx >= len(parts) {
			continue
		}

		path := strings.TrimSpace(parts[pathIdx])
		if path == "" {
			continue
		}
		statuses[path] = code
	}
	return statuses, nil
}

func parsePorcelainPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.Contains(path, " -> ") {
		parts := strings.Split(path, " -> ")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return strings.TrimSpace(path)
}

func normalizeStatusCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if code == "??" {
		return "?"
	}

	// porcelain uses XY: prefer unstaged (Y) for worktree-like display, then X.
	if len(code) >= 2 {
		if normalized := normalizeStatusRune(rune(code[1])); normalized != "" {
			return normalized
		}
		if normalized := normalizeStatusRune(rune(code[0])); normalized != "" {
			return normalized
		}
	}
	return normalizeStatusRune(rune(code[0]))
}

func normalizeStatusRune(r rune) string {
	switch r {
	case 'M':
		return "M"
	case 'A':
		return "A"
	case 'D':
		return "D"
	case 'R', 'C':
		return "R"
	case '?':
		return "?"
	default:
		return ""
	}
}

func FileDiff(mode Mode, algo DiffAlgo, file string) (string, error) {
	if mode == Staged {
		return loadDiffStaged(algo, file)
	}
	return loadDiffWorktree(algo, file)
}

func loadDiffWorktree(algo DiffAlgo, file string) (string, error) {
	args := append([]string{"diff", "--no-color", "--unified=3"}, diffAlgoArgs(algo)...)
	args = append(args, "--", file)
	out, err := runDiffWithAlgoFallback(algo, args...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return out, nil
	}

	untracked, err := isUntrackedFile(file)
	if err != nil {
		return "", err
	}
	if !untracked {
		return out, nil
	}

	// Untracked files are not shown by plain `git diff`; compare against /dev/null.
	return loadDiffNoIndex(algo, file)
}

func loadDiffStaged(algo DiffAlgo, file string) (string, error) {
	args := append([]string{"diff", "--cached", "--no-color", "--unified=3"}, diffAlgoArgs(algo)...)
	args = append(args, "--", file)
	return runDiffWithAlgoFallback(algo, args...)
}

func loadDiffNoIndex(algo DiffAlgo, file string) (string, error) {
	args := append([]string{"diff", "--no-color", "--unified=3"}, diffAlgoArgs(algo)...)
	args = append(args, "--no-index", "--", "/dev/null", file)
	return runDiffAllowExitCodesWithAlgoFallback(algo, map[int]struct{}{1: {}}, args...)
}

func diffAlgoArgs(algo DiffAlgo) []string {
	switch algo {
	case DiffHistogram:
		return []string{"--histogram"}
	case DiffPatience:
		return []string{"--patience"}
	default:
		return nil
	}
}

func runDiffWithAlgoFallback(algo DiffAlgo, args ...string) (string, error) {
	out, err := runGit(args...)
	if err == nil {
		return out, nil
	}
	if !shouldFallbackToDefaultAlgo(err, algo) {
		return "", err
	}

	fallback := removeDiffAlgoFlag(args)
	return runGit(fallback...)
}

func runDiffAllowExitCodesWithAlgoFallback(algo DiffAlgo, allowed map[int]struct{}, args ...string) (string, error) {
	out, err := runGitAllowExitCodes(allowed, args...)
	if err == nil {
		return out, nil
	}
	if !shouldFallbackToDefaultAlgo(err, algo) {
		return "", err
	}

	fallback := removeDiffAlgoFlag(args)
	return runGitAllowExitCodes(allowed, fallback...)
}

func shouldFallbackToDefaultAlgo(err error, algo DiffAlgo) bool {
	if algo == DiffDefault || err == nil {
		return false
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		return false
	}
	out := strings.ToLower(cmdErr.Output)
	if out == "" {
		return false
	}

	mentionsAlgo := strings.Contains(out, "--histogram") ||
		strings.Contains(out, "--patience") ||
		strings.Contains(out, "histogram") ||
		strings.Contains(out, "patience")
	if !mentionsAlgo {
		return false
	}

	return strings.Contains(out, "unknown option") ||
		strings.Contains(out, "unrecognized option") ||
		strings.Contains(out, "invalid option") ||
		strings.Contains(out, "usage: git diff")
}

func removeDiffAlgoFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--histogram" || arg == "--patience" {
			continue
		}
		out = append(out, arg)
	}
	return out
}

type CommandError struct {
	Args   []string
	Output string
	Err    error
}

func (e *CommandError) Error() string {
	if strings.TrimSpace(e.Output) != "" {
		return e.Output
	}
	return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
}

func FriendlyError(err error) string {
	if err == nil {
		return ""
	}

	var cmdErr *CommandError
	if errors.As(err, &cmdErr) {
		lower := strings.ToLower(cmdErr.Output)
		if strings.Contains(lower, "not a git repository") {
			return "Not a git repository. Run TDiff inside a git repository."
		}
	}
	return err.Error()
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return output, &CommandError{
			Args:   append([]string(nil), args...),
			Output: output,
			Err:    err,
		}
	}
	return stdout.String(), nil
}

func runGitAllowExitCodes(allowed map[int]struct{}, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if _, ok := allowed[exitErr.ExitCode()]; ok {
				return stdout.String(), nil
			}
		}
		output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return output, &CommandError{
			Args:   append([]string(nil), args...),
			Output: output,
			Err:    err,
		}
	}
	return stdout.String(), nil
}

func isUntrackedFile(file string) (bool, error) {
	out, err := runGit("ls-files", "--others", "--exclude-standard", "--", file)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func parseNonEmptyLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func appendUnique(base []string, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	for _, item := range base {
		seen[item] = struct{}{}
	}
	for _, item := range extra {
		if _, exists := seen[item]; exists {
			continue
		}
		base = append(base, item)
		seen[item] = struct{}{}
	}
	return base
}
