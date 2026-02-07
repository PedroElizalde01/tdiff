package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PedroElizalde01/tdiff/diff"
	"github.com/charmbracelet/lipgloss"
)

type Focus int

const (
	FocusFiles Focus = iota
	FocusOld
	FocusNew
)

func (f Focus) String() string {
	switch f {
	case FocusOld:
		return "old"
	case FocusNew:
		return "new"
	default:
		return "files"
	}
}

type RenderModel struct {
	Width         int
	Height        int
	ModeLabel     string
	Focus         Focus
	Files         []string
	Selected      int
	SidebarScroll int
	Rows          []diff.Row
	Cursor        int
	DiffScroll    int
	SelectedFile  string
	Error         string
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	titleStyle  = lipgloss.NewStyle().Bold(true)

	selectedFocusedStyle   = lipgloss.NewStyle().Bold(true).Reverse(true)
	selectedUnfocusedStyle = lipgloss.NewStyle().Bold(true)

	metaStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	delStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	addStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("236"))

	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func Render(m RenderModel) string {
	if m.Width <= 0 || m.Height <= 0 {
		return ""
	}
	if len(m.Files) == 0 {
		m.Files = []string{"(no changes)"}
	}
	if len(m.Rows) == 0 {
		m.Rows = []diff.Row{{Old: "(no diff)", New: "(no diff)", Kind: diff.Meta}}
	}

	headerText := fmt.Sprintf("TDiff | mode: %s | focus: %s", strings.ToUpper(m.ModeLabel), m.Focus.String())
	if m.SelectedFile != "" {
		headerText += " | file: " + m.SelectedFile
	}
	if m.Error != "" {
		headerText += " | error: " + m.Error
	}
	headerLine := headerStyle.Render(fitWidth(headerText, m.Width))

	bodyHeight := m.Height - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	sidebarWidth := calcSidebarWidth(m.Width)
	mainWidth := m.Width - sidebarWidth - 2
	if mainWidth < 4 {
		mainWidth = 4
		sidebarWidth = m.Width - mainWidth - 2
		if sidebarWidth < 1 {
			sidebarWidth = 1
		}
	}

	leftPaneWidth := (mainWidth - 1) / 2
	rightPaneWidth := mainWidth - 1 - leftPaneWidth
	if leftPaneWidth < 1 {
		leftPaneWidth = 1
	}
	if rightPaneWidth < 1 {
		rightPaneWidth = 1
	}

	sidebar := renderSidebar(m, sidebarWidth, bodyHeight)
	oldPane, newPane := renderPanes(m, leftPaneWidth, rightPaneWidth, bodyHeight)
	sep := separatorStyle.Render("│")
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, sep, oldPane, sep, newPane)

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, body)
}

func renderSidebar(m RenderModel, width, height int) string {
	lines := make([]string, 0, height)
	lines = append(lines, titleStyle.Render(fitWidth("CHANGES", width)))

	listHeight := height - 1
	if listHeight < 1 {
		return strings.Join(lines, "\n")
	}

	for i := 0; i < listHeight; i++ {
		idx := m.SidebarScroll + i
		line := ""
		if idx >= 0 && idx < len(m.Files) {
			line = m.Files[idx]
		}
		line = fitWidth(line, width)

		if idx == m.Selected {
			if m.Focus == FocusFiles {
				line = selectedFocusedStyle.Render(line)
			} else {
				line = selectedUnfocusedStyle.Render(line)
			}
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func renderPanes(m RenderModel, leftWidth, rightWidth, height int) (string, string) {
	oldLines := make([]string, 0, height)
	newLines := make([]string, 0, height)
	oldLines = append(oldLines, titleStyle.Render(fitWidth("OLD", leftWidth)))
	newLines = append(newLines, titleStyle.Render(fitWidth("NEW", rightWidth)))

	contentHeight := height - 1
	if contentHeight < 1 {
		return strings.Join(oldLines, "\n"), strings.Join(newLines, "\n")
	}

	oldNoWidth := lineNumberWidth(m.Rows, true)
	newNoWidth := lineNumberWidth(m.Rows, false)
	showCursor := m.Focus == FocusOld || m.Focus == FocusNew

	for i := 0; i < contentHeight; i++ {
		idx := m.DiffScroll + i
		if idx < 0 || idx >= len(m.Rows) {
			oldLines = append(oldLines, fitWidth("", leftWidth))
			newLines = append(newLines, fitWidth("", rightWidth))
			continue
		}

		row := m.Rows[idx]
		cursor := showCursor && idx == m.Cursor
		oldLines = append(oldLines, renderPaneLine(row, true, oldNoWidth, leftWidth, cursor))
		newLines = append(newLines, renderPaneLine(row, false, newNoWidth, rightWidth, cursor))
	}

	return strings.Join(oldLines, "\n"), strings.Join(newLines, "\n")
}

func renderPaneLine(row diff.Row, oldPane bool, noWidth, width int, cursor bool) string {
	var no *int
	text := ""
	if oldPane {
		no = row.OldNo
		text = row.Old
	} else {
		no = row.NewNo
		text = row.New
	}

	noText := ""
	if no != nil {
		noText = strconv.Itoa(*no)
	}
	line := fmt.Sprintf("%*s %s", noWidth, noText, text)
	line = fitWidth(line, width)

	style := paneStyle(row, oldPane)
	if cursor {
		style = style.Copy().Inherit(cursorStyle)
	}
	return style.Render(line)
}

func paneStyle(row diff.Row, oldPane bool) lipgloss.Style {
	switch row.Kind {
	case diff.Meta:
		return metaStyle
	case diff.Hunk:
		return hunkStyle
	}

	if oldPane {
		if row.OldNo != nil && row.NewNo == nil {
			return delStyle
		}
		if row.OldNo != nil && row.NewNo != nil && row.Old != row.New {
			return delStyle
		}
		return lipgloss.NewStyle()
	}

	if row.NewNo != nil && row.OldNo == nil {
		return addStyle
	}
	if row.NewNo != nil && row.OldNo != nil && row.Old != row.New {
		return addStyle
	}
	return lipgloss.NewStyle()
}

func lineNumberWidth(rows []diff.Row, old bool) int {
	maxNo := 0
	for i := range rows {
		if old {
			if rows[i].OldNo != nil && *rows[i].OldNo > maxNo {
				maxNo = *rows[i].OldNo
			}
		} else {
			if rows[i].NewNo != nil && *rows[i].NewNo > maxNo {
				maxNo = *rows[i].NewNo
			}
		}
	}
	if maxNo < 1 {
		return 3
	}
	width := len(strconv.Itoa(maxNo))
	if width < 3 {
		return 3
	}
	return width
}

func calcSidebarWidth(totalWidth int) int {
	width := 32
	if totalWidth < 90 {
		width = 28
	}
	if totalWidth > 140 {
		width = 36
	}
	maxAllowed := totalWidth - 20
	if maxAllowed < 16 {
		maxAllowed = 16
	}
	if width > maxAllowed {
		width = maxAllowed
	}
	if width < 16 {
		width = 16
	}
	return width
}

func fitWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		if width == 1 {
			return string(r[:1])
		}
		return string(r[:width-1]) + "…"
	}
	if len(r) < width {
		return s + strings.Repeat(" ", width-len(r))
	}
	return s
}
