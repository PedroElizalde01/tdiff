package diff

import (
	"regexp"
	"sort"
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

const (
	editPairSimilarityThreshold = 0.45
	editPairComparisonLimit     = 10_000
)

type blockRow struct {
	delIdx int
	addIdx int
}

type pairCandidate struct {
	delIdx   int
	addIdx   int
	score    float64
	distance int
}

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

		pairs := alignEditRows(dels, adds)
		for _, p := range pairs {
			row := Row{Kind: Context}
			if p.delIdx >= 0 {
				row.OldNo = intPtr(oldLine)
				row.Old = dels[p.delIdx]
				oldLine++
			}
			if p.addIdx >= 0 {
				row.NewNo = intPtr(newLine)
				row.New = adds[p.addIdx]
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
			if isHiddenFileHeaderMeta(line) {
				continue
			}
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

func alignEditRows(dels, adds []string) []blockRow {
	if len(dels) == 0 {
		return makeSingleSideRows(false, len(adds))
	}
	if len(adds) == 0 {
		return makeSingleSideRows(true, len(dels))
	}

	if len(dels)*len(adds) > editPairComparisonLimit {
		return alignEditRowsByIndex(dels, adds)
	}

	matches := greedyMatchPairs(dels, adds, editPairSimilarityThreshold)
	if len(matches) == 0 {
		return alignUnmatchedRows(dels, adds)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].delIdx != matches[j].delIdx {
			return matches[i].delIdx < matches[j].delIdx
		}
		return matches[i].addIdx < matches[j].addIdx
	})

	rows := make([]blockRow, 0, len(dels)+len(adds))
	nextDel := 0
	nextAdd := 0
	for _, match := range matches {
		for i := nextDel; i < match.delIdx; i++ {
			rows = append(rows, blockRow{delIdx: i, addIdx: -1})
		}
		for j := nextAdd; j < match.addIdx; j++ {
			rows = append(rows, blockRow{delIdx: -1, addIdx: j})
		}
		rows = append(rows, blockRow{delIdx: match.delIdx, addIdx: match.addIdx})
		nextDel = match.delIdx + 1
		nextAdd = match.addIdx + 1
	}
	for i := nextDel; i < len(dels); i++ {
		rows = append(rows, blockRow{delIdx: i, addIdx: -1})
	}
	for j := nextAdd; j < len(adds); j++ {
		rows = append(rows, blockRow{delIdx: -1, addIdx: j})
	}
	return rows
}

func alignEditRowsByIndex(dels, adds []string) []blockRow {
	n := len(dels)
	if len(adds) > n {
		n = len(adds)
	}
	rows := make([]blockRow, 0, n)
	for i := 0; i < n; i++ {
		row := blockRow{delIdx: -1, addIdx: -1}
		if i < len(dels) {
			row.delIdx = i
		}
		if i < len(adds) {
			row.addIdx = i
		}
		rows = append(rows, row)
	}
	return rows
}

func alignUnmatchedRows(dels, adds []string) []blockRow {
	rows := make([]blockRow, 0, len(dels)+len(adds))
	for i := range dels {
		rows = append(rows, blockRow{delIdx: i, addIdx: -1})
	}
	for j := range adds {
		rows = append(rows, blockRow{delIdx: -1, addIdx: j})
	}
	return rows
}

func makeSingleSideRows(oldSide bool, n int) []blockRow {
	rows := make([]blockRow, 0, n)
	for i := 0; i < n; i++ {
		if oldSide {
			rows = append(rows, blockRow{delIdx: i, addIdx: -1})
		} else {
			rows = append(rows, blockRow{delIdx: -1, addIdx: i})
		}
	}
	return rows
}

func greedyMatchPairs(dels, adds []string, minScore float64) []blockRow {
	delTokens := make([][]string, len(dels))
	for i := range dels {
		delTokens[i] = Tokenize(strings.TrimSpace(dels[i]))
	}
	addTokens := make([][]string, len(adds))
	for j := range adds {
		addTokens[j] = Tokenize(strings.TrimSpace(adds[j]))
	}

	candidates := make([]pairCandidate, 0, len(dels)*len(adds))
	for i := range dels {
		for j := range adds {
			score := SimilarityTokens(delTokens[i], addTokens[j])
			if score < minScore {
				continue
			}
			distance := i - j
			if distance < 0 {
				distance = -distance
			}
			candidates = append(candidates, pairCandidate{
				delIdx:   i,
				addIdx:   j,
				score:    score,
				distance: distance,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].delIdx != candidates[j].delIdx {
			return candidates[i].delIdx < candidates[j].delIdx
		}
		return candidates[i].addIdx < candidates[j].addIdx
	})

	usedDel := make([]bool, len(dels))
	usedAdd := make([]bool, len(adds))
	matches := make([]blockRow, 0, minInt(len(dels), len(adds)))
	for _, candidate := range candidates {
		if usedDel[candidate.delIdx] || usedAdd[candidate.addIdx] {
			continue
		}
		match := blockRow{delIdx: candidate.delIdx, addIdx: candidate.addIdx}
		if crossesExisting(match, matches) {
			continue
		}
		usedDel[candidate.delIdx] = true
		usedAdd[candidate.addIdx] = true
		matches = append(matches, match)
	}
	return matches
}

func crossesExisting(next blockRow, matches []blockRow) bool {
	for _, match := range matches {
		if next.delIdx < match.delIdx && next.addIdx > match.addIdx {
			return true
		}
		if next.delIdx > match.delIdx && next.addIdx < match.addIdx {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func isHiddenFileHeaderMeta(line string) bool {
	return strings.HasPrefix(line, "diff --git ") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "--- ") ||
		strings.HasPrefix(line, "+++ ")
}

func intPtr(v int) *int {
	n := v
	return &n
}
