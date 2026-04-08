package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Colors & styles
// ---------------------------------------------------------------------------

var (
	green  = lipgloss.Color("#4ade80")
	red    = lipgloss.Color("#f87171")
	dim    = lipgloss.Color("#52525b")
	white  = lipgloss.Color("#e4e4e7")
	bright = lipgloss.Color("#fafafa")
	orange = lipgloss.Color("#e68e0d")
	cyan   = lipgloss.Color("#5f9ea0")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(bright)
	subtitleStyle = lipgloss.NewStyle().Foreground(dim)
	labelStyle    = lipgloss.NewStyle().Foreground(dim)
	valueStyle    = lipgloss.NewStyle().Foreground(white)
	selectedStyle = lipgloss.NewStyle().Foreground(bright).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(white)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	footerStyle   = lipgloss.NewStyle().Foreground(dim)
	greenStyle    = lipgloss.NewStyle().Foreground(green)
	orangeStyle   = lipgloss.NewStyle().Foreground(orange)
	cyanStyle     = lipgloss.NewStyle().Foreground(cyan)
)

// Silence unused-variable warnings for colours reserved for future use.
var _ = red
var _ = subtitleStyle
var _ = orangeStyle
var _ = cyanStyle

// ---------------------------------------------------------------------------
// Data types — swarm files
// ---------------------------------------------------------------------------

type AgentInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type SwarmFile struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Topology string      `json:"topology"`
	Agents   []AgentInfo `json:"agents"`
}

// ---------------------------------------------------------------------------
// Data types — SSH devices (hosts.json)
// ---------------------------------------------------------------------------

type Device struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	User        string `json:"user"`
	Port        int    `json:"port"`
	OS          string `json:"os"`
	Description string `json:"description"`
}

// ---------------------------------------------------------------------------
// Tea messages
// ---------------------------------------------------------------------------

type tickMsg time.Time

type statusMsg struct {
	sessions     []string
	swarmCount   int
	agentTotal   int
	agentActive  int
	devices      []Device
	err          error
}

type connectMsg struct {
	name      string
	reachable bool
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	sessions     []string
	swarmCount   int
	agentTotal   int
	agentActive  int
	devices      []Device
	deviceStatus map[string]bool
	cursor       int
	spinner      spinner.Model
	loading      bool
	err          error
	width        int
	height       int
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		deviceStatus: make(map[string]bool),
		spinner:      s,
		loading:      true,
	}
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func fetchCmd() tea.Cmd {
	return func() tea.Msg {
		var msg statusMsg

		// 1. Tmux sessions
		out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line != "" {
					msg.sessions = append(msg.sessions, line)
				}
			}
		}

		// 2. Swarm data
		home, _ := os.UserHomeDir()
		swarmGlob := filepath.Join(home, ".local", "share", "omacmux", "swarms", "*", "swarm.json")
		matches, _ := filepath.Glob(swarmGlob)
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err != nil {
				continue
			}
			var sf SwarmFile
			if json.Unmarshal(data, &sf) != nil {
				continue
			}
			msg.swarmCount++
			msg.agentTotal += len(sf.Agents)
			for _, a := range sf.Agents {
				if a.Status == "active" {
					msg.agentActive++
				}
			}
		}

		// 3. Devices
		hostsPath := "/Users/aadarwal/omacmux/mesh/hosts.json"
		data, err := os.ReadFile(hostsPath)
		if err == nil {
			_ = json.Unmarshal(data, &msg.devices)
		}

		return msg
	}
}

func checkConnectivity(d Device) tea.Cmd {
	return func() tea.Msg {
		port := fmt.Sprintf("%d", d.Port)
		err := exec.Command("nc", "-z", "-w", "1", d.Host, port).Run()
		return connectMsg{name: d.Name, reachable: err == nil}
	}
}

func scheduleTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func sshCmd(d Device) tea.Cmd {
	return func() tea.Msg {
		target := d.User + "@" + d.Host
		_ = exec.Command("tmux", "new-window", "-n", d.Name, "ssh", target).Start()
		return nil
	}
}

// ---------------------------------------------------------------------------
// Bubble Tea interface
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if len(m.devices) > 0 {
				m.cursor = (m.cursor + 1) % len(m.devices)
			}
		case "k", "up":
			if len(m.devices) > 0 {
				m.cursor = (m.cursor - 1 + len(m.devices)) % len(m.devices)
			}
		case "enter":
			if len(m.devices) > 0 {
				return m, sshCmd(m.devices[m.cursor])
			}
		case "r":
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, fetchCmd())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, fetchCmd())

	case statusMsg:
		m.sessions = msg.sessions
		m.swarmCount = msg.swarmCount
		m.agentTotal = msg.agentTotal
		m.agentActive = msg.agentActive
		m.devices = msg.devices
		m.err = msg.err
		m.loading = false
		// Reset status map and fire connectivity checks in parallel.
		m.deviceStatus = make(map[string]bool)
		var cmds []tea.Cmd
		cmds = append(cmds, scheduleTick())
		for _, d := range m.devices {
			cmds = append(cmds, checkConnectivity(d))
		}
		return m, tea.Batch(cmds...)

	case connectMsg:
		m.deviceStatus[msg.name] = msg.reachable

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Loading state
	if m.loading {
		b.WriteString("\n  " + m.spinner.View() + " Loading...\n")
		return b.String()
	}

	// ── Status section ──────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("Status") + "\n")
	b.WriteString("  " + dimStyle.Render("──────────────────────────────────") + "\n")

	sessionCount := len(m.sessions)
	b.WriteString("  " + labelStyle.Render("Sessions") + "   " +
		valueStyle.Render(fmt.Sprintf("%d active", sessionCount)) + "\n")

	swarmDetail := fmt.Sprintf("%d active", m.swarmCount)
	if m.agentTotal > 0 {
		swarmDetail += fmt.Sprintf("  (%d agents)", m.agentTotal)
	}
	b.WriteString("  " + labelStyle.Render("Swarms") + "     " +
		valueStyle.Render(swarmDetail) + "\n")

	// ── Devices section ─────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("Devices") + "\n")
	b.WriteString("  " + dimStyle.Render("──────────────────────────────────") + "\n")

	if len(m.devices) == 0 {
		b.WriteString("  " + dimStyle.Render("no devices configured") + "\n")
	} else {
		for i, d := range m.devices {
			cursor := "  "
			if i == m.cursor {
				cursor = "▸ "
			}

			dot := dimStyle.Render("○")
			if m.deviceStatus[d.Name] {
				dot = greenStyle.Render("●")
			}

			name := normalStyle.Render(d.Name)
			desc := dimStyle.Render(d.Description)
			if i == m.cursor {
				name = selectedStyle.Render(d.Name)
				desc = normalStyle.Render(d.Description)
			}

			// Pad name to 16 chars for alignment.
			padded := d.Name
			if len(padded) < 16 {
				padded += strings.Repeat(" ", 16-len(padded))
			}
			if i == m.cursor {
				padded = selectedStyle.Render(padded)
			} else {
				padded = name
				if len(d.Name) < 16 {
					padded = normalStyle.Render(d.Name) + strings.Repeat(" ", 16-len(d.Name))
				}
			}

			b.WriteString(cursor + dot + " " + padded + desc + "\n")
		}
	}

	// ── Footer ──────────────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString("  " + footerStyle.Render("↑↓/jk=nav  ⏎=ssh  r=refresh  q=quit") + "\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
