package diff

import "unicode"

type OpKind int

const (
	Equal OpKind = iota
	Delete
	Insert
)

type Op struct {
	Kind OpKind
	Tok  string
}

func Tokenize(s string) []string {
	if s == "" {
		return nil
	}

	runes := []rune(s)
	start := 0
	current := tokenClass(runes[0])
	tokens := make([]string, 0, len(runes))

	for i := 1; i < len(runes); i++ {
		next := tokenClass(runes[i])
		if next != current {
			tokens = append(tokens, string(runes[start:i]))
			start = i
			current = next
		}
	}
	tokens = append(tokens, string(runes[start:]))
	return tokens
}

func DiffTokens(a, b []string) []Op {
	n := len(a)
	m := len(b)
	if n == 0 && m == 0 {
		return nil
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
				continue
			}
			if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	ops := make([]Op, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			ops = append(ops, Op{Kind: Equal, Tok: a[i]})
			i++
			j++
			continue
		}

		if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, Op{Kind: Delete, Tok: a[i]})
			i++
		} else {
			ops = append(ops, Op{Kind: Insert, Tok: b[j]})
			j++
		}
	}
	for i < n {
		ops = append(ops, Op{Kind: Delete, Tok: a[i]})
		i++
	}
	for j < m {
		ops = append(ops, Op{Kind: Insert, Tok: b[j]})
		j++
	}

	return ops
}

func tokenClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
		return 1
	default:
		return 2
	}
}
