package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/luanzeba/gh-csd/internal/gh"
	"golang.org/x/term"
)

type codespacesMsg struct {
	codespaces []gh.Codespace
}

type errMsg struct {
	err error
}

type actionFinishedMsg struct {
	action string
	name   string
	err    error
}

// Model drives the codespaces TUI.
type Model struct {
	codespaces       []gh.Codespace
	cursor           int
	offset           int
	width            int
	height           int
	styles           Styles
	status           string
	statusIsError    bool
	loading          bool
	loadErr          error
	confirmingDelete bool
	deleteTarget     string
}

// NewModel creates a TUI model with default styling.
func NewModel() Model {
	theme := DefaultTheme()
	styles := NewStyles(theme)
	return Model{
		styles:  styles,
		loading: true,
	}
}

// Init runs the initial codespace load.
func (m Model) Init() tea.Cmd {
	return fetchCodespacesCmd()
}

// Update handles messages and input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		return m, nil
	case codespacesMsg:
		m.loading = false
		m.loadErr = nil
		selected := m.selectedName()
		m.codespaces = msg.codespaces
		m.setCursorByName(selected)
		if m.status == "" || m.statusIsError {
			m.status = fmt.Sprintf("%d codespace(s)", len(m.codespaces))
			m.statusIsError = false
		}
		return m, nil
	case errMsg:
		m.loading = false
		m.loadErr = msg.err
		m.status = msg.err.Error()
		m.statusIsError = true
		return m, nil
	case actionFinishedMsg:
		m.confirmingDelete = false
		if msg.action == "ssh" {
			if msg.err != nil {
				fmt.Fprintln(os.Stderr, msg.err)
			}
			return m, tea.Quit
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %v", actionLabel(msg.action), msg.err)
			m.statusIsError = true
		} else if msg.action == "delete" {
			m.status = fmt.Sprintf("Deleted %s", msg.name)
			m.statusIsError = false
		}
		m.loading = true
		return m, fetchCodespacesCmd()
	case tea.KeyMsg:
		if m.confirmingDelete {
			switch msg.String() {
			case "y", "Y":
				name := m.deleteTarget
				m.confirmingDelete = false
				m.status = fmt.Sprintf("Deleting %s...", name)
				m.statusIsError = false
				return m, deleteCodespaceCmd(name)
			case "n", "N", "esc", "enter":
				m.confirmingDelete = false
				m.deleteTarget = ""
				m.status = "Delete cancelled"
				m.statusIsError = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			m.status = "Refreshing codespaces..."
			m.statusIsError = false
			m.loading = true
			return m, fetchCodespacesCmd()
		case "up", "k":
			m.moveCursor(-1)
			return m, nil
		case "down", "j":
			m.moveCursor(1)
			return m, nil
		case "s":
			if selected := m.selectedCodespace(); selected != nil {
				m.status = fmt.Sprintf("Connecting to %s...", selected.Name)
				m.statusIsError = false
				return m, sshCodespaceCmd(selected.Name)
			}
		case "d":
			if selected := m.selectedCodespace(); selected != nil {
				m.confirmingDelete = true
				m.deleteTarget = selected.Name
				return m, nil
			}
		}
	}

	return m, nil
}

// View renders the UI.
func (m Model) View() string {
	status := m.statusView()
	body := m.bodyView()
	footer := m.footerView()

	sections := make([]string, 0, 3)
	if status != "" {
		sections = append(sections, status)
	}
	sections = append(sections, body, footer)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) statusView() string {
	if m.status == "" {
		return ""
	}
	if m.statusIsError {
		return m.styles.StatusError.Render(m.status)
	}
	return m.styles.Status.Render(m.status)
}

func (m Model) bodyView() string {
	switch {
	case m.loading:
		return m.styles.Empty.Render("Loading codespaces...")
	case m.loadErr != nil:
		return m.styles.Empty.Render("Failed to load codespaces. Press r to retry.")
	case len(m.codespaces) == 0:
		return m.styles.Empty.Render("No codespaces found.")
	default:
		return m.tableView()
	}
}

func (m Model) footerView() string {
	if m.confirmingDelete {
		prompt := fmt.Sprintf("Delete %s? [y/N]", m.deleteTarget)
		return m.styles.Confirm.Render(prompt)
	}
	return m.styles.Help.Render("↑/↓/j/k navigate • s ssh • d delete • r refresh • q quit")
}

func (m Model) tableView() string {
	specs := columnsForWidth(m.effectiveWidth())
	header := m.styles.Header.Render(renderHeader(specs))

	visibleRows := m.visibleRows()
	maxOffset := max(0, len(m.codespaces)-visibleRows)
	start := clamp(m.offset, 0, maxOffset)
	end := min(start+visibleRows, len(m.codespaces))

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		row := renderRow(specs, m.codespaces[i])
		style := m.styles.Row
		if i == m.cursor {
			style = m.styles.SelectedRow
		}
		rows = append(rows, style.Render(row))
	}

	lines := append([]string{header}, rows...)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) moveCursor(delta int) {
	if len(m.codespaces) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, len(m.codespaces)-1)
	m.ensureCursorVisible()
}

func (m *Model) setCursorByName(name string) {
	if len(m.codespaces) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if name != "" {
		for i, cs := range m.codespaces {
			if cs.Name == name {
				m.cursor = i
				m.ensureCursorVisible()
				return
			}
		}
	}
	m.cursor = clamp(m.cursor, 0, len(m.codespaces)-1)
	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	if len(m.codespaces) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	visibleRows := m.visibleRows()
	if visibleRows < 1 {
		visibleRows = 1
	}
	maxOffset := max(0, len(m.codespaces)-visibleRows)
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
	m.offset = clamp(m.offset, 0, maxOffset)
}

func (m Model) visibleRows() int {
	rows := m.tableHeight() - 1
	if rows < 1 {
		return 1
	}
	return rows
}

func (m Model) tableHeight() int {
	height := m.effectiveHeight()
	if height == 0 {
		return 10
	}
	chrome := 2
	height -= chrome
	if height < 3 {
		return 3
	}
	return height
}

func (m Model) effectiveWidth() int {
	if m.width > 0 {
		return m.width
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && width > 0 {
		return width
	}
	return 120
}

func (m Model) effectiveHeight() int {
	if m.height > 0 {
		return m.height
	}
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && height > 0 {
		return height
	}
	return 0
}

func (m Model) selectedCodespace() *gh.Codespace {
	if len(m.codespaces) == 0 {
		return nil
	}
	if m.cursor < 0 || m.cursor >= len(m.codespaces) {
		return nil
	}
	return &m.codespaces[m.cursor]
}

func (m Model) selectedName() string {
	selected := m.selectedCodespace()
	if selected == nil {
		return ""
	}
	return selected.Name
}

func actionLabel(action string) string {
	switch action {
	case "ssh":
		return "SSH"
	case "delete":
		return "Delete"
	default:
		return action
	}
}

func fetchCodespacesCmd() tea.Cmd {
	return func() tea.Msg {
		codespaces, err := gh.ListCodespaces()
		if err != nil {
			return errMsg{err: err}
		}
		sort.SliceStable(codespaces, func(i, j int) bool {
			return codespaces[i].CreatedAt.After(codespaces[j].CreatedAt)
		})
		return codespacesMsg{codespaces: codespaces}
	}
}

func sshCodespaceCmd(name string) tea.Cmd {
	return tea.ExecProcess(buildCommand("gh", "csd", "ssh", "-c", name), func(err error) tea.Msg {
		return actionFinishedMsg{action: "ssh", name: name, err: err}
	})
}

func deleteCodespaceCmd(name string) tea.Cmd {
	return tea.ExecProcess(buildCommand("gh", "cs", "delete", "-c", name), func(err error) tea.Msg {
		return actionFinishedMsg{action: "delete", name: name, err: err}
	})
}

func buildCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
