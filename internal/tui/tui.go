// Package tui is the interactive live port view behind `localpilot tui`.
// It owns no data access: the caller supplies the loader and killer, so it
// shows exactly what `list` shows and kills exactly the way `kill` does.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/output"
)

type mode int

const (
	modeList mode = iota
	modeDetail
	modeConfirmKill
)

type (
	tickMsg   struct{}
	loadedMsg struct {
		ports []models.Port
		err   error
	}
	killedMsg struct{ err error }
)

type model struct {
	load     func(ctx context.Context) ([]models.Port, error) // ports to display, already filtered/annotated
	kill     func(ctx context.Context, pid int32) error       // terminates the process behind a port
	interval time.Duration

	ports  []models.Port
	cursor int
	mode   mode
	err    error
	notice string
	width  int
	height int
}

// Run starts the TUI and blocks until the user quits.
func Run(load func(context.Context) ([]models.Port, error), kill func(context.Context, int32) error, interval time.Duration) error {
	_, err := tea.NewProgram(&model{load: load, kill: kill, interval: interval}, tea.WithAltScreen()).Run()
	return err
}

func (m *model) Init() tea.Cmd { return m.fetch() }

func (m *model) fetch() tea.Cmd {
	return func() tea.Msg {
		ports, err := m.load(context.Background())
		return loadedMsg{ports, err}
	}
}

func (m *model) tick() tea.Cmd {
	return tea.Tick(m.interval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) selected() *models.Port {
	if m.cursor < 0 || m.cursor >= len(m.ports) {
		return nil
	}
	return &m.ports[m.cursor]
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		return m, m.fetch()
	case loadedMsg:
		m.err = msg.err
		if msg.err == nil {
			m.ports = msg.ports
			m.cursor = max(0, min(m.cursor, len(m.ports)-1))
			if m.mode != modeList && m.selected() == nil {
				m.mode = modeList
			}
		}
		return m, m.tick()
	case killedMsg:
		m.mode = modeList
		if msg.err != nil {
			m.notice = "kill failed: " + msg.err.Error()
		} else {
			m.notice = "killed"
		}
		return m, m.fetch()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.mode == modeConfirmKill {
		p := m.selected()
		if key == "y" && p != nil {
			pid := p.PID
			return m, func() tea.Msg { return killedMsg{m.kill(context.Background(), pid)} }
		}
		m.mode = modeList
		m.notice = ""
		return m, nil
	}

	m.notice = ""
	switch key {
	case "q":
		return m, tea.Quit
	case "esc", "backspace":
		m.mode = modeList
	case "j", "down":
		m.cursor = min(m.cursor+1, len(m.ports)-1)
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "enter":
		if m.selected() != nil {
			m.mode = modeDetail
		}
	case "x":
		p := m.selected()
		switch {
		case p == nil:
		case p.Container != nil:
			m.notice = "port is published by container " + p.Container.Name + "; use: localpilot kill --container"
		case p.PID == 0:
			m.notice = "no owning process to kill"
		default:
			m.mode = modeConfirmKill
		}
	}
	return m, nil
}

func (m *model) View() string {
	var b strings.Builder
	b.WriteString("LOCALPILOT  ")
	b.WriteString(fmt.Sprintf("%d ports · refresh %s\n\n", len(m.ports), m.interval))

	if m.mode == modeDetail {
		b.WriteString(detail(m.selected()))
	} else {
		m.writeTable(&b)
	}

	b.WriteString("\n")
	switch {
	case m.mode == modeConfirmKill:
		p := m.selected()
		fmt.Fprintf(&b, "Kill %s (PID %d) on port %d? [y/N]", name(*p), p.PID, p.Number)
	case m.err != nil:
		fmt.Fprintf(&b, "error: %v", m.err)
	case m.notice != "":
		b.WriteString(m.notice)
	case m.mode == modeDetail:
		b.WriteString("esc back · x kill · q quit")
	default:
		b.WriteString("j/k move · enter inspect · x kill · q quit")
	}
	return b.String()
}

func (m *model) writeTable(b *strings.Builder) {
	if len(m.ports) == 0 {
		b.WriteString("No listening ports found.\n")
		return
	}
	fmt.Fprintf(b, "  %-20s %-14s %-7s %-8s %-12s %s\n", "ADDRESS", "PROCESS", "PORT", "PID", "CONTAINER", "STATUS")

	// Keep the cursor row visible in short terminals: header(2) + footer(2).
	rows := len(m.ports)
	start := 0
	if m.height > 5 && rows > m.height-4 {
		rows = m.height - 4
		start = min(max(0, m.cursor-rows+1), len(m.ports)-rows)
	}
	for i := start; i < start+rows; i++ {
		p := m.ports[i]
		cursor, on, off := " ", "", ""
		if i == m.cursor {
			cursor, on, off = ">", "\x1b[7m", "\x1b[0m"
		}
		status := "running"
		if p.Stale {
			status = "STALE"
		}
		container := "-"
		if p.Container != nil {
			container = p.Container.Name
		} else if p.WSL != nil {
			container = "wsl:" + p.WSL.Distro
		}
		pid := "-"
		if p.PID > 0 {
			pid = fmt.Sprint(p.PID)
		}
		fmt.Fprintf(b, "%s%s %-20s %-14s %-7d %-8s %-12s %s%s\n", on, cursor,
			output.Truncate(fmt.Sprintf("%s:%d", p.Address, p.Number), 20), output.Truncate(name(p), 14), p.Number, pid, output.Truncate(container, 12), status, off)
	}
}

func name(p models.Port) string {
	if p.Process != nil {
		return p.Process.Name
	}
	return "-"
}

func detail(p *models.Port) string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	row := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%-10s %s\n", k, v)
		}
	}
	row("Port", fmt.Sprintf("%s:%d (%s)", p.Address, p.Number, p.Protocol))
	if p.Process != nil {
		pr := p.Process
		row("Process", fmt.Sprintf("%s (PID %d)", pr.Name, pr.PID))
		row("Command", pr.Command)
		row("Cwd", pr.Cwd)
		row("Parent", pr.ParentName)
		row("Started", output.FormatTime(pr.StartTime))
		row("Memory", output.FormatBytes(pr.MemoryBytes))
	}
	if p.Container != nil {
		row("Container", p.Container.Name+" ("+p.Container.Image+")")
	}
	if p.WSL != nil {
		row("WSL", p.WSL.Distro)
	}
	if p.Stale {
		row("Status", "STALE (old, idle, no established connections)")
	}
	return b.String()
}
