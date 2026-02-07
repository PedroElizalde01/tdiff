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

	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}, nil
	}

	lines := strings.Split(out, "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func FileDiff(mode Mode, file string) (string, error) {
	args := []string{"diff", "--no-color", "--unified=0", "--", file}
	if mode == Staged {
		args = []string{"diff", "--cached", "--no-color", "--unified=0", "--", file}
	}
	return runGit(args...)
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
