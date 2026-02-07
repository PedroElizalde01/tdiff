package diff

import (
	"regexp"
	"strconv"
	"strings"
)

type Kind int

const (
	Meta Kind = iota
	Hunk
	Del
	Add
	Context
)

type Row struct {
	OldNo *int
	NewNo *int
	Old   string
	New   string
	Kind  Kind
}

var hunkHeaderRE = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func ParseUnified(input string) ([]Row, []int) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}

	if strings.Contains(input, "Binary files") && strings.Contains(input, " differ") {
		msg := "(binary file changed)"
		return []Row{{Old: msg, New: msg, Kind: Meta}}, nil
	}

	lines := strings.Split(strings.TrimRight(input, "\n"), "\n")
	rows := make([]Row, 0, len(lines))
	hunkStarts := make([]int, 0, 8)

	var oldLine int
	var newLine int
	inHunk := false

	dels := make([]string, 0, 8)
	adds := make([]string, 0, 8)

	flushEdits := func() {
		if len(dels) == 0 && len(adds) == 0 {
			return
		}
		n := len(dels)
		if len(adds) > n {
			n = len(adds)
		}
		for i := 0; i < n; i++ {
			row := Row{Kind: Context}
			if i < len(dels) {
				row.OldNo = intPtr(oldLine)
				row.Old = dels[i]
				oldLine++
			}
			if i < len(adds) {
				row.NewNo = intPtr(newLine)
				row.New = adds[i]
				newLine++
			}
			if row.OldNo != nil && row.NewNo == nil {
				row.Kind = Del
			}
			if row.NewNo != nil && row.OldNo == nil {
				row.Kind = Add
			}
			rows = append(rows, row)
		}
		dels = dels[:0]
		adds = adds[:0]
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@ "):
			flushEdits()
			oldLine, newLine = parseHunkHeader(line)
			inHunk = true
			rows = append(rows, Row{Old: line, New: line, Kind: Hunk})
			hunkStarts = append(hunkStarts, len(rows)-1)
		case !inHunk && isMetaLine(line):
			flushEdits()
			inHunk = false
			rows = append(rows, Row{Old: line, New: line, Kind: Meta})
		default:
			if !inHunk {
				rows = append(rows, Row{Old: line, New: line, Kind: Meta})
				continue
			}
			if line == "" {
				flushEdits()
				rows = append(rows, Row{Old: "", New: "", Kind: Context})
				continue
			}
			switch line[0] {
			case '-':
				dels = append(dels, line[1:])
			case '+':
				adds = append(adds, line[1:])
			case ' ':
				flushEdits()
				rows = append(rows, Row{
					OldNo: intPtr(oldLine),
					NewNo: intPtr(newLine),
					Old:   line[1:],
					New:   line[1:],
					Kind:  Context,
				})
				oldLine++
				newLine++
			case '\\':
				flushEdits()
				rows = append(rows, Row{Old: line, New: line, Kind: Meta})
			default:
				flushEdits()
				rows = append(rows, Row{Old: line, New: line, Kind: Meta})
			}
		}
	}

	flushEdits()
	return rows, hunkStarts
}

func parseHunkHeader(line string) (int, int) {
	m := hunkHeaderRE.FindStringSubmatch(line)
	if len(m) < 3 {
		return 1, 1
	}
	oldStart, err := strconv.Atoi(m[1])
	if err != nil {
		oldStart = 1
	}
	newStart, err := strconv.Atoi(m[2])
	if err != nil {
		newStart = 1
	}
	return oldStart, newStart
}

func isMetaLine(line string) bool {
	prefixes := []string{
		"diff --git ",
		"index ",
		"--- ",
		"+++ ",
		"new file mode ",
		"deleted file mode ",
		"similarity index ",
		"rename from ",
		"rename to ",
		"old mode ",
		"new mode ",
		"Binary files ",
		"GIT binary patch",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

func intPtr(v int) *int {
	n := v
	return &n
}
