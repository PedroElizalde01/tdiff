package main

import (
	"fmt"

	"github.com/PedroElizalde01/tdiff/diff"
	"github.com/PedroElizalde01/tdiff/git"
	"github.com/PedroElizalde01/tdiff/ui"
	tea "github.com/charmbracelet/bubbletea"
)

type filesLoadedMsg struct {
	req   int
	mode  git.Mode
	files []string
	err   error
}

type diffLoadedMsg struct {
	req        int
	mode       git.Mode
	file       string
	rows       []diff.Row
	hunkStarts []int
	err        error
}

type model struct {
	mode          git.Mode
	focus         ui.Focus
	files         []string
	selected      int
	noChanges     bool
	rows          []diff.Row
	hunkStarts    []int
	cursor        int
	cursors       map[string]int
	sidebarScroll int
	diffScroll    int
	width         int
	height        int
	errMsg        string
	filesReq      int
	diffReq       int
}

func initialModel() model {
	return model{
		mode:      git.Worktree,
		focus:     ui.FocusFiles,
		files:     []string{"(loading...)"},
		rows:      loadingRows("loading..."),
		cursors:   map[string]int{},
		width:     120,
		height:    32,
		filesReq:  1,
		noChanges: false,
	}
}

func (m model) Init() tea.Cmd {
	return loadFilesCmd(m.mode, m.filesReq)
}

func loadFilesCmd(mode git.Mode, req int) tea.Cmd {
	return func() tea.Msg {
		files, err := git.ListChangedFiles(mode)
		return filesLoadedMsg{
			req:   req,
			mode:  mode,
			files: files,
			err:   err,
		}
	}
}

func loadDiffCmd(mode git.Mode, file string, req int) tea.Cmd {
	return func() tea.Msg {
		raw, err := git.FileDiff(mode, file)
		if err != nil {
			return diffLoadedMsg{
				req:  req,
				mode: mode,
				file: file,
				err:  err,
			}
		}
		rows, hunks := diff.ParseUnified(raw)
		return diffLoadedMsg{
			req:        req,
			mode:       mode,
			file:       file,
			rows:       rows,
			hunkStarts: hunks,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSidebarVisible()
		m.ensureCursorVisible()
		return m, nil
	case filesLoadedMsg:
		if msg.req != m.filesReq || msg.mode != m.mode {
			return m, nil
		}
		if msg.err != nil {
			m.errMsg = git.FriendlyError(msg.err)
			m.noChanges = true
			m.files = []string{"(no changes)"}
			m.selected = 0
			m.rows = noDiffRows()
			m.hunkStarts = nil
			m.cursor = 0
			m.sidebarScroll = 0
			m.diffScroll = 0
			return m, nil
		}

		prevFile := m.selectedFile()
		m.errMsg = ""
		if len(msg.files) == 0 {
			m.noChanges = true
			m.files = []string{"(no changes)"}
			m.selected = 0
			m.rows = noDiffRows()
			m.hunkStarts = nil
			m.cursor = 0
			m.sidebarScroll = 0
			m.diffScroll = 0
			return m, nil
		}

		m.noChanges = false
		m.files = msg.files
		m.selected = clamp(m.selected, 0, len(m.files)-1)
		if prevFile != "" {
			if idx := indexOf(prevFile, m.files); idx >= 0 {
				m.selected = idx
			}
		}
		m.ensureSidebarVisible()

		m.rows = loadingRows("loading diff...")
		m.hunkStarts = nil
		m.diffScroll = 0
		m.cursor = 0

		file := m.selectedFile()
		if file == "" {
			m.rows = noDiffRows()
			return m, nil
		}
		m.diffReq++
		return m, loadDiffCmd(m.mode, file, m.diffReq)
	case diffLoadedMsg:
		if msg.req != m.diffReq || msg.mode != m.mode || msg.file != m.selectedFile() {
			return m, nil
		}
		if msg.err != nil {
			m.errMsg = git.FriendlyError(msg.err)
			m.rows = noDiffRows()
			m.hunkStarts = nil
			m.cursor = 0
			m.diffScroll = 0
			return m, nil
		}

		m.errMsg = ""
		m.rows = msg.rows
		m.hunkStarts = msg.hunkStarts
		if len(m.rows) == 0 {
			m.rows = noDiffRows()
			m.hunkStarts = nil
		}

		current := m.selectedFile()
		m.cursor = clamp(m.cursors[current], 0, len(m.rows)-1)
		m.diffScroll = 0
		m.ensureCursorVisible()
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "s":
			m.saveCursor()
			m.mode = m.mode.Toggle()
			m.noChanges = false
			m.files = []string{"(loading...)"}
			m.selected = 0
			m.rows = loadingRows("loading...")
			m.hunkStarts = nil
			m.cursor = 0
			m.sidebarScroll = 0
			m.diffScroll = 0
			m.errMsg = ""
			m.filesReq++
			return m, loadFilesCmd(m.mode, m.filesReq)
		}

		switch m.focus {
		case ui.FocusFiles:
			switch key {
			case "up", "k":
				cmd := m.moveSelection(-1)
				return m, cmd
			case "down", "j":
				cmd := m.moveSelection(1)
				return m, cmd
			case "enter", "right":
				m.focus = ui.FocusOld
				return m, nil
			}
		case ui.FocusOld:
			switch key {
			case "up", "k":
				m.moveCursor(-1)
			case "down", "j":
				m.moveCursor(1)
			case "left":
				m.focus = ui.FocusFiles
			case "right":
				m.focus = ui.FocusNew
			case "n":
				m.jumpHunk(1)
			case "p":
				m.jumpHunk(-1)
			case "g":
				m.goTop()
			case "G":
				m.goBottom()
			}
			return m, nil
		case ui.FocusNew:
			switch key {
			case "up", "k":
				m.moveCursor(-1)
			case "down", "j":
				m.moveCursor(1)
			case "left":
				m.focus = ui.FocusOld
			case "right":
				// no-op by spec
			case "n":
				m.jumpHunk(1)
			case "p":
				m.jumpHunk(-1)
			case "g":
				m.goTop()
			case "G":
				m.goBottom()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m model) View() string {
	return ui.Render(ui.RenderModel{
		Width:         m.width,
		Height:        m.height,
		ModeLabel:     m.mode.String(),
		Focus:         m.focus,
		Files:         m.files,
		Selected:      m.selected,
		SidebarScroll: m.sidebarScroll,
		Rows:          m.rows,
		Cursor:        m.cursor,
		DiffScroll:    m.diffScroll,
		SelectedFile:  m.selectedFile(),
		Error:         m.errMsg,
	})
}

func (m *model) moveSelection(delta int) tea.Cmd {
	if !m.hasRealFiles() {
		return nil
	}

	m.saveCursor()
	next := clamp(m.selected+delta, 0, len(m.files)-1)
	if next == m.selected {
		return nil
	}

	m.selected = next
	m.ensureSidebarVisible()
	file := m.selectedFile()
	if file == "" {
		return nil
	}

	m.rows = loadingRows("loading diff...")
	m.hunkStarts = nil
	m.cursor = 0
	m.diffScroll = 0
	m.diffReq++
	return loadDiffCmd(m.mode, file, m.diffReq)
}

func (m *model) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, len(m.rows)-1)
	m.saveCursor()
	m.ensureCursorVisible()
}

func (m *model) jumpHunk(direction int) {
	if len(m.hunkStarts) == 0 {
		return
	}

	if direction > 0 {
		for _, idx := range m.hunkStarts {
			if idx > m.cursor {
				m.cursor = idx
				m.saveCursor()
				m.ensureCursorVisible()
				return
			}
		}
		return
	}

	for i := len(m.hunkStarts) - 1; i >= 0; i-- {
		if m.hunkStarts[i] < m.cursor {
			m.cursor = m.hunkStarts[i]
			m.saveCursor()
			m.ensureCursorVisible()
			return
		}
	}
}

func (m *model) goTop() {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = 0
	m.saveCursor()
	m.ensureCursorVisible()
}

func (m *model) goBottom() {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = len(m.rows) - 1
	m.saveCursor()
	m.ensureCursorVisible()
}

func (m *model) saveCursor() {
	file := m.selectedFile()
	if file == "" {
		return
	}
	m.cursors[file] = m.cursor
}

func (m *model) hasRealFiles() bool {
	if m.noChanges || len(m.files) == 0 {
		return false
	}
	if len(m.files) == 1 && m.files[0] == "(loading...)" {
		return false
	}
	return true
}

func (m *model) selectedFile() string {
	if !m.hasRealFiles() || m.selected < 0 || m.selected >= len(m.files) {
		return ""
	}
	return m.files[m.selected]
}

func (m *model) bodyHeight() int {
	if m.height <= 1 {
		return 1
	}
	return m.height - 1
}

func (m *model) ensureSidebarVisible() {
	if len(m.files) == 0 {
		m.sidebarScroll = 0
		return
	}

	visible := m.bodyHeight() - 1
	if visible < 1 {
		visible = 1
	}

	if m.selected < m.sidebarScroll {
		m.sidebarScroll = m.selected
	}
	if m.selected >= m.sidebarScroll+visible {
		m.sidebarScroll = m.selected - visible + 1
	}

	maxScroll := len(m.files) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.sidebarScroll = clamp(m.sidebarScroll, 0, maxScroll)
}

func (m *model) ensureCursorVisible() {
	if len(m.rows) == 0 {
		m.cursor = 0
		m.diffScroll = 0
		return
	}

	m.cursor = clamp(m.cursor, 0, len(m.rows)-1)
	visible := m.bodyHeight() - 1
	if visible < 1 {
		visible = 1
	}

	if m.cursor < m.diffScroll {
		m.diffScroll = m.cursor
	}
	if m.cursor >= m.diffScroll+visible {
		m.diffScroll = m.cursor - visible + 1
	}

	maxScroll := len(m.rows) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.diffScroll = clamp(m.diffScroll, 0, maxScroll)
}

func noDiffRows() []diff.Row {
	return []diff.Row{{Old: "(no diff)", New: "(no diff)", Kind: diff.Meta}}
}

func loadingRows(message string) []diff.Row {
	return []diff.Row{{Old: fmt.Sprintf("(%s)", message), New: fmt.Sprintf("(%s)", message), Kind: diff.Meta}}
}

func indexOf(needle string, list []string) int {
	for i := range list {
		if list[i] == needle {
			return i
		}
	}
	return -1
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
