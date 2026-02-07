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

func ListChangedFiles(mode Mode) ([]string, error) {
	args := []string{"diff", "--name-only"}
	if mode == Staged {
		args = []string{"diff", "--cached", "--name-only"}
	}

	out, err := runGit(args...)
	if err != nil {
		return nil, err
	}

	files := parseNonEmptyLines(out)
	if mode == Worktree {
		untrackedOut, err := runGit("ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		files = appendUnique(files, parseNonEmptyLines(untrackedOut))
	}
	return files, nil
}

func FileDiff(mode Mode, file string) (string, error) {
	args := []string{"diff", "--no-color", "--unified=3", "--", file}
	if mode == Staged {
		args = []string{"diff", "--cached", "--no-color", "--unified=3", "--", file}
		return runGit(args...)
	}

	out, err := runGit(args...)
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
	return runGitAllowExitCodes(map[int]struct{}{1: {}}, "diff", "--no-color", "--unified=3", "--no-index", "--", "/dev/null", file)
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
