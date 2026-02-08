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
	AlgoLabel     string
	Focus         Focus
	Files         []string
	FileStatuses  map[string]string
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

	metaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hunkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	contextStyle = lipgloss.NewStyle()
	oldLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	newLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cursorStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236"))

	oldWordHighlight = lipgloss.NewStyle().Background(lipgloss.Color("52")).Foreground(lipgloss.Color("255"))
	newWordHighlight = lipgloss.NewStyle().Background(lipgloss.Color("22")).Foreground(lipgloss.Color("255"))

	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	borderDimStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
	borderHotStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("7"))

	sidebarBannerTopPadding    = 1
	sidebarBannerBottomPadding = 1
	sidebarBannerLines         = []string{
		"████████╗██████╗ ██╗███████╗███████╗",
		"╚══██╔══╝██╔══██╗██║██╔════╝██╔════╝",
		"   ██║   ██║  ██║██║█████╗  █████╗  ",
		"   ██║   ██║  ██║██║██╔══╝  ██╔══╝  ",
		"   ██║   ██████╔╝██║██║     ██║     ",
		"   ╚═╝   ╚═════╝ ╚═╝╚═╝     ╚═╝     ",
	}
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

	headerText := fmt.Sprintf("TDiff | mode: %s | algo: %s | focus: %s", strings.ToUpper(m.ModeLabel), strings.ToLower(m.AlgoLabel), m.Focus.String())
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
	mainWidth := m.Width - sidebarWidth
	if mainWidth < 4 {
		mainWidth = 4
		sidebarWidth = m.Width - mainWidth
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

	paneContentHeight := bodyHeight - 2
	if paneContentHeight < 1 {
		paneContentHeight = 1
	}
	oldContentWidth := leftPaneWidth - 2
	if oldContentWidth < 1 {
		oldContentWidth = 1
	}
	newContentWidth := rightPaneWidth - 2
	if newContentWidth < 1 {
		newContentWidth = 1
	}

	oldPaneContent, newPaneContent := renderPanes(m, oldContentWidth, newContentWidth, paneContentHeight)
	oldPane := sectionBorder(m.Focus == FocusOld).Render(fitBlock(oldPaneContent, oldContentWidth, paneContentHeight))
	newPane := sectionBorder(m.Focus == FocusNew).Render(fitBlock(newPaneContent, newContentWidth, paneContentHeight))

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, oldPane, newPane)

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, body)
}

func renderSidebar(m RenderModel, width, height int) string {
	if height <= 0 {
		return ""
	}

	bannerBoxHeight, filesBoxHeight := splitSidebarHeights(height)
	filesContentWidth := width - 2
	if filesContentWidth < 1 {
		filesContentWidth = 1
	}

	sections := make([]string, 0, 2)
	if bannerBoxHeight > 0 {
		banner := fitBlock(renderBannerContent(width, bannerBoxHeight), width, bannerBoxHeight)
		sections = append(sections, banner)
	}

	if filesBoxHeight > 0 {
		filesContentHeight := filesBoxHeight - 2
		if filesContentHeight < 1 {
			filesContentHeight = 1
		}
		files := sectionBorder(m.Focus == FocusFiles).Render(fitBlock(renderFilesContent(m, filesContentWidth, filesContentHeight), filesContentWidth, filesContentHeight))
		sections = append(sections, files)
	}
	if len(sections) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderBannerContent(width, height int) string {
	lines := make([]string, 0, height)
	for i := 0; i < sidebarBannerTopPadding && len(lines) < height; i++ {
		lines = append(lines, fitWidth("", width))
	}
	for _, bannerLine := range sidebarBannerLines {
		if len(lines) >= height {
			break
		}
		lines = append(lines, fitWidth(bannerLine, width))
	}
	for i := 0; i < sidebarBannerBottomPadding && len(lines) < height; i++ {
		lines = append(lines, fitWidth("", width))
	}
	for len(lines) < height {
		lines = append(lines, fitWidth("", width))
	}
	return strings.Join(lines, "\n")
}

func renderFilesContent(m RenderModel, width, height int) string {
	lines := make([]string, 0, height)
	lines = append(lines, titleStyle.Render(fitWidth("FILES CHANGED", width)))
	listHeight := height - 1
	if listHeight < 0 {
		listHeight = 0
	}

	for i := 0; i < listHeight; i++ {
		idx := m.SidebarScroll + i
		line := ""
		if idx >= 0 && idx < len(m.Files) {
			line = renderSidebarFile(m.Files[idx], m.FileStatuses[m.Files[idx]])
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
	for len(lines) < height {
		lines = append(lines, fitWidth("", width))
	}
	return strings.Join(lines, "\n")
}

func renderSidebarFile(path, status string) string {
	if path == "(loading...)" || path == "(no changes)" {
		return path
	}
	label := statusLabel(status)
	return statusStyle.Render("["+label+"]") + " " + path
}

func statusLabel(status string) string {
	switch status {
	case "M":
		return "M"
	case "A":
		return "A"
	case "D":
		return "D"
	case "R":
		return "R"
	case "?":
		return "U"
	default:
		return "·"
	}
}

func SidebarVisibleFiles(sidebarHeight int) int {
	if sidebarHeight <= 0 {
		return 0
	}

	_, filesHeight := splitSidebarHeights(sidebarHeight)
	if filesHeight < 3 {
		return 0
	}
	contentHeight := filesHeight - 2
	if contentHeight < 1 {
		return 0
	}
	listHeight := contentHeight - 1
	if listHeight < 0 {
		return 0
	}
	return listHeight
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func splitSidebarHeights(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}

	minFilesHeight := 3 // border + at least title
	if total <= minFilesHeight {
		return 0, total
	}

	idealBanner := sidebarBannerTopPadding + len(sidebarBannerLines) + sidebarBannerBottomPadding
	maxBanner := total - minFilesHeight
	banner := intMin(idealBanner, maxBanner)
	if banner < 0 {
		banner = 0
	}
	files := total - banner
	return banner, files
}

func fitBlock(content string, width, height int) string {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = fitWidth(lines[i], width)
	}
	for len(lines) < height {
		lines = append(lines, fitWidth("", width))
	}
	return strings.Join(lines, "\n")
}

func sectionBorder(focused bool) lipgloss.Style {
	if focused {
		return borderHotStyle
	}
	return borderDimStyle
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
		oldText := row.Old
		newText := row.New
		if isEditRow(row) {
			oldText, newText = inlineHighlight(row.Old, row.New)
		}

		oldLines = append(oldLines, renderPaneLine(row, oldText, row.OldNo, oldNoWidth, leftWidth, cursor, true))
		newLines = append(newLines, renderPaneLine(row, newText, row.NewNo, newNoWidth, rightWidth, cursor, false))
	}

	return strings.Join(oldLines, "\n"), strings.Join(newLines, "\n")
}

func renderPaneLine(row diff.Row, text string, no *int, noWidth, width int, cursor bool, oldPane bool) string {
	noText := ""
	if no != nil {
		noText = strconv.Itoa(*no)
	}
	style := paneStyle(row, oldPane)
	text = style.Render(text)
	line := formatPaneCell(noText, text, noWidth, width)

	if cursor {
		line = cursorStyle.Render(line)
	}
	return line
}

func paneStyle(row diff.Row, oldPane bool) lipgloss.Style {
	switch row.Kind {
	case diff.Meta:
		return metaStyle
	case diff.Hunk:
		return hunkStyle
	case diff.Context:
		return contextStyle
	}

	if oldPane {
		if isPureDeletion(row) {
			return oldLineStyle
		}
		if isEditRow(row) {
			return contextStyle
		}
		return contextStyle
	}

	if isPureAddition(row) {
		return newLineStyle
	}
	if isEditRow(row) {
		return contextStyle
	}
	return contextStyle
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
	width := 34
	if totalWidth < 90 {
		width = 30
	}
	if totalWidth > 140 {
		width = 38
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
	return lipgloss.NewStyle().MaxWidth(width).Width(width).Render(s)
}

func formatPaneCell(noText, text string, noWidth, width int) string {
	prefix := fmt.Sprintf("%*s ", noWidth, noText)
	contentWidth := width - lipgloss.Width(prefix)
	if contentWidth < 0 {
		contentWidth = 0
	}
	text = lipgloss.NewStyle().MaxWidth(contentWidth).Render(text)
	return fitWidth(prefix+text, width)
}

func isEditRow(row diff.Row) bool {
	if row.Kind == diff.Meta || row.Kind == diff.Hunk {
		return false
	}
	if row.Old == "" || row.New == "" {
		return false
	}
	return row.Old != row.New
}

func inlineHighlight(oldText, newText string) (string, string) {
	ops := diff.DiffTokens(diff.Tokenize(oldText), diff.Tokenize(newText))
	var oldBuilder strings.Builder
	var newBuilder strings.Builder

	for _, op := range ops {
		switch op.Kind {
		case diff.Equal:
			oldBuilder.WriteString(oldLineStyle.Render(op.Tok))
			newBuilder.WriteString(newLineStyle.Render(op.Tok))
		case diff.Delete:
			oldBuilder.WriteString(oldWordHighlight.Render(op.Tok))
		case diff.Insert:
			newBuilder.WriteString(newWordHighlight.Render(op.Tok))
		}
	}

	return oldBuilder.String(), newBuilder.String()
}

func isPureDeletion(row diff.Row) bool {
	return row.OldNo != nil && row.NewNo == nil
}

func isPureAddition(row diff.Row) bool {
	return row.NewNo != nil && row.OldNo == nil
}
