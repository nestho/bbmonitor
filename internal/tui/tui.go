package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bbmonitor/bbmonitor/internal/fetcher"
	"github.com/bbmonitor/bbmonitor/internal/storage"
)

type Model struct {
	store    *storage.Storage
	spinner  spinner.Model
	viewport viewport.Model
	width    int
	height   int
	ready    bool
	running  bool
	lastRun  time.Time
	results  []fetcher.Result
	logs     []string
	stats    stats
	quitting bool
}

type stats struct {
	Programs int64
	Targets  int64
	Pending  int64
}

type tickMsg time.Time
type syncDoneMsg []fetcher.Result
type statsMsg stats
type logMsg string

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1)
	headerStyle = lipgloss.NewStyle().Bold(true)
)

func New(store *storage.Storage) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return Model{store: store, spinner: s, logs: make([]string, 0, 128)}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.refreshStats(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) refreshStats() tea.Cmd {
	return func() tea.Msg {
		p, t, c, _ := m.store.Stats()
		return statsMsg{Programs: p, Targets: t, Pending: c}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			if !m.running {
				m.running = true
				m.addLog("manual sync requested")
			}
		case "s":
			return m, m.refreshStats()
		case "c":
			m.logs = m.logs[:0]
			m.viewport.SetContent("")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h := msg.Height - 14
		if h < 5 {
			h = 5
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, h)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = h
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tickMsg:
		return m, tea.Batch(tickCmd(), m.refreshStats())
	case statsMsg:
		m.stats = stats(msg)
	case logMsg:
		m.addLog(string(msg))
	case syncDoneMsg:
		m.running = false
		m.lastRun = time.Now()
		m.results = []fetcher.Result(msg)
		for _, r := range m.results {
			if r.Error != nil {
				m.addLog(fmt.Sprintf("%-16s  ERR  %v", r.Source, r.Error))
			} else {
				m.addLog(fmt.Sprintf("%-16s  +%d prog  +%d tgt", r.Source, r.ProgramsNew, r.TargetsNew))
			}
		}
		m.addLog("--- sync finished ---")
		return m, m.refreshStats()
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) addLog(line string) {
	ts := time.Now().Format("15:04:05")
	m.logs = append(m.logs, fmt.Sprintf("%s  %s", ts, line))
	if len(m.logs) > 300 {
		m.logs = m.logs[len(m.logs)-300:]
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return " loading…"
	}
	status := "idle"
	spin := " "
	if m.running {
		status = "syncing"
		spin = m.spinner.View()
	}
	title := titleStyle.Render("bbmonitor")
	statusPart := dimStyle.Render(fmt.Sprintf("%s %s", spin, status))
	statsLine := fmt.Sprintf("programs %-6d  targets %-7d  pending changes %-4d", m.stats.Programs, m.stats.Targets, m.stats.Pending)
	last := "never"
	if !m.lastRun.IsZero() {
		last = m.lastRun.Format("15:04:05")
	}
	var srcLines []string
	if len(m.results) > 0 {
		for _, r := range m.results {
			mark := okStyle.Render("✓")
			extra := fmt.Sprintf("+%d/+%d", r.ProgramsNew, r.TargetsNew)
			if r.Error != nil {
				mark = errStyle.Render("✗")
				extra = "error"
			}
			srcLines = append(srcLines, fmt.Sprintf("  %s %-14s  %s", mark, r.Source, extra))
		}
	} else {
		srcLines = append(srcLines, dimStyle.Render("  (no sync yet)"))
	}
	help := dimStyle.Render("q quit   r force sync   s refresh stats   c clear log")
	header := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", statusPart),
		statsLine,
		dimStyle.Render("last sync: "+last),
		"",
		headerStyle.Render("sources"),
		strings.Join(srcLines, "\n"),
		"",
		help,
	)
	body := boxStyle.Width(m.width - 4).Render(m.viewport.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body)
}

func Log(m *Model, msg string) tea.Cmd {
	return func() tea.Msg { return logMsg(msg) }
}

func SyncDone(results []fetcher.Result) tea.Msg {
	return syncDoneMsg(results)
}
