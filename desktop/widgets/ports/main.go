package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Colors & styles (omacmux matte-black multi-accent theme)
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
	selectedStyle = lipgloss.NewStyle().Foreground(bright).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(white)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	footerStyle   = lipgloss.NewStyle().Foreground(dim)
	greenStyle    = lipgloss.NewStyle().Foreground(green)
	orangeStyle   = lipgloss.NewStyle().Foreground(orange)
	cyanStyle     = lipgloss.NewStyle().Foreground(cyan)
	sectionStyle  = lipgloss.NewStyle().Foreground(orange).Bold(true)
	errStyle      = lipgloss.NewStyle().Foreground(red)

	_ = normalStyle
	_ = errStyle
)

// ---------------------------------------------------------------------------
// Known services & categories
// ---------------------------------------------------------------------------

var knownPorts = map[int]string{
	80:    "HTTP",
	443:   "HTTPS",
	1433:  "MSSQL",
	3000:  "Next.js",
	3001:  "Dev Server",
	4000:  "Phoenix",
	4200:  "Angular",
	5000:  "Flask",
	5173:  "Vite",
	5432:  "PostgreSQL",
	3306:  "MySQL",
	6379:  "Redis",
	8080:  "HTTP Alt",
	8443:  "HTTPS Alt",
	8888:  "Jupyter",
	9090:  "Prometheus",
	27017: "MongoDB",
	26257: "CockroachDB",
}

const (
	catDev = "Dev Servers"
	catDB  = "Databases"
	catSys = "System"
)

func categorize(port int) string {
	switch {
	case port == 5432 || port == 3306 || port == 6379 || port == 27017 ||
		port == 26257 || port == 1433:
		return catDB
	case port >= 3000 && port <= 9999:
		return catDev
	default:
		return catSys
	}
}

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

type Port struct {
	Number   int
	Process  string
	PID      string
	Label    string
	Category string
}

// ---------------------------------------------------------------------------
// Tea messages
// ---------------------------------------------------------------------------

type tickMsg time.Time

type portsMsg struct {
	ports []Port
	err   error
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	ports   []Port
	cursor  int
	spinner spinner.Model
	loading bool
	err     error
	width   int
	height  int
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{spinner: s, loading: true}
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func fetchPorts() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n").Output()
		if err != nil && len(out) == 0 {
			return portsMsg{}
		}

		seen := make(map[int]Port)
		for _, line := range strings.Split(string(out), "\n")[1:] {
			fields := strings.Fields(line)
			if len(fields) < 9 {
				continue
			}

			nameField := fields[8] // e.g. *:3000 or 127.0.0.1:5432
			idx := strings.LastIndex(nameField, ":")
			if idx < 0 {
				continue
			}
			portNum, err := strconv.Atoi(nameField[idx+1:])
			if err != nil {
				continue
			}

			if _, exists := seen[portNum]; exists {
				continue
			}

			label := knownPorts[portNum]
			seen[portNum] = Port{
				Number:   portNum,
				Process:  fields[0],
				PID:      fields[1],
				Label:    label,
				Category: categorize(portNum),
			}
		}

		ports := make([]Port, 0, len(seen))
		for _, p := range seen {
			ports = append(ports, p)
		}

		catOrder := map[string]int{catDev: 0, catDB: 1, catSys: 2}
		sort.Slice(ports, func(i, j int) bool {
			ci, cj := catOrder[ports[i].Category], catOrder[ports[j].Category]
			if ci != cj {
				return ci < cj
			}
			return ports[i].Number < ports[j].Number
		})

		return portsMsg{ports: ports}
	}
}

func scheduleTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Bubble Tea interface
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchPorts(), scheduleTick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.ports)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.cursor < len(m.ports) {
				p := m.ports[m.cursor]
				if p.Number >= 3000 && p.Number <= 9999 {
					_ = exec.Command("open", fmt.Sprintf("http://localhost:%d", p.Number)).Start()
				}
			}
		case "r":
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, fetchPorts())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, fetchPorts()

	case portsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.ports = msg.ports
			m.err = nil
			if m.cursor >= len(m.ports) {
				m.cursor = max(0, len(m.ports)-1)
			}
		}
		return m, scheduleTick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 40
	}

	// ── Header ──────────────────────────────────────────────────────────
	title := titleStyle.Render("  Ports")
	count := ""
	if len(m.ports) > 0 {
		count = subtitleStyle.Render(fmt.Sprintf("%d listening", len(m.ports)))
	}
	pad := w - lipgloss.Width(title) - lipgloss.Width(count)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(title + strings.Repeat(" ", pad) + count + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", max(0, w-4))) + "\n")

	// ── Loading ─────────────────────────────────────────────────────────
	if m.loading && len(m.ports) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Scanning ports...\n")
		return b.String()
	}

	if len(m.ports) == 0 {
		b.WriteString("\n  " + dimStyle.Render("no listening ports") + "\n")
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("  r=refresh  q=quit") + "\n")
		return b.String()
	}

	// ── Port list (grouped) ─────────────────────────────────────────────
	lastCat := ""
	flatIdx := 0
	for _, p := range m.ports {
		if p.Category != lastCat {
			if lastCat != "" {
				b.WriteString("\n")
			}
			b.WriteString("  " + sectionStyle.Render(p.Category) + "\n")
			lastCat = p.Category
		}

		selected := flatIdx == m.cursor
		prefix := "  "
		if selected {
			prefix = "▸ "
		}

		dot := greenStyle.Render("●")
		portStr := fmt.Sprintf(":%d", p.Number)

		label := ""
		if p.Label != "" {
			label = " " + dimStyle.Render("("+p.Label+")")
		}

		if selected {
			b.WriteString(prefix + dot + " " + selectedStyle.Render(fmt.Sprintf("%-7s", portStr)) + " " + orangeStyle.Render(p.Process) + label + "\n")
		} else {
			b.WriteString(prefix + dot + " " + cyanStyle.Render(fmt.Sprintf("%-7s", portStr)) + " " + dimStyle.Render(p.Process) + label + "\n")
		}
		flatIdx++
	}

	// ── Footer ──────────────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓/jk=nav  ⏎=open  r=refresh  q=quit") + "\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
