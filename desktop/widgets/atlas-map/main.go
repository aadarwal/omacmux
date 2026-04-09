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
// Styles
// ---------------------------------------------------------------------------

var (
	green  = lipgloss.Color("#4ade80")
	red    = lipgloss.Color("#f87171")
	dim    = lipgloss.Color("#52525b")
	white  = lipgloss.Color("#e4e4e7")
	bright = lipgloss.Color("#fafafa")
	orange = lipgloss.Color("#e68e0d")
	cyan   = lipgloss.Color("#5f9ea0")
	purple = lipgloss.Color("#c084fc")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(bright)
	selectedStyle = lipgloss.NewStyle().Foreground(bright).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(white)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	footerStyle   = lipgloss.NewStyle().Foreground(dim)
	greenStyle    = lipgloss.NewStyle().Foreground(green)
	cyanStyle     = lipgloss.NewStyle().Foreground(cyan)
	orangeStyle   = lipgloss.NewStyle().Foreground(orange)
	errStyle      = lipgloss.NewStyle().Foreground(red)
	sectionStyle  = lipgloss.NewStyle().Foreground(orange).Bold(true)
	purpleStyle   = lipgloss.NewStyle().Foreground(purple)
)

// ---------------------------------------------------------------------------
// Atlas types
// ---------------------------------------------------------------------------

type AtlasState struct {
	GeneratedAt string         `json:"generated_at"`
	Pinned      []PinnedItem   `json:"pinned"`
	Sections    []AtlasSection `json:"sections"`
}

type PinnedItem struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type AtlasSection struct {
	Section string       `json:"section"`
	Label   string       `json:"label"`
	Entries []AtlasEntry `json:"entries"`
}

type AtlasEntry struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Branch      string `json:"branch,omitempty"`
	LastCommit  string `json:"last_commit,omitempty"`
	Dirty       string `json:"dirty,omitempty"`
	HasGithub   string `json:"has_github,omitempty"`
	Host        string `json:"host,omitempty"`
	User        string `json:"user,omitempty"`
	OS          string `json:"os,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	LocalPath   string `json:"local_path,omitempty"`
}

type DisplayItem struct {
	IsHeader    bool
	SectionName string
	EntryCount  int // for headers: count of entries in section
	Entry       AtlasEntry
	IsPinned    bool
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type tickMsg time.Time
type atlasMsg struct {
	items []DisplayItem
	err   error
}
type connectMsg struct {
	name      string
	reachable bool
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	items        []DisplayItem
	cursor       int
	scroll       int // first visible line
	spinner      spinner.Model
	loading      bool
	loadErr      error
	deviceStatus map[string]bool
	width        int
	height       int
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:      s,
		loading:      true,
		deviceStatus: make(map[string]bool),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadCmd(), scheduleTick())
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func getOmacmuxPath() string {
	if p := os.Getenv("OMACMUX_PATH"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "omacmux")
}

func getStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "omacmux", "atlas", "state.json")
}

func loadCmd() tea.Cmd {
	return func() tea.Msg {
		statePath := getStatePath()
		data, err := os.ReadFile(statePath)
		if err != nil || len(data) == 0 {
			// Generate fresh
			omx := getOmacmuxPath()
			atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
			data, err = exec.Command("bash", "-c",
				fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, atlasFile)).Output()
			if err != nil {
				return atlasMsg{err: fmt.Errorf("atlas: %w", err)}
			}
		} else {
			// Refresh in background if stale
			if info, serr := os.Stat(statePath); serr == nil {
				if time.Since(info.ModTime()) > 5*time.Minute {
					go func() {
						omx := getOmacmuxPath()
						atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
						exec.Command("bash", "-c",
							fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, atlasFile)).Run()
					}()
				}
			}
		}

		var state AtlasState
		if err := json.Unmarshal(data, &state); err != nil {
			return atlasMsg{err: fmt.Errorf("parse: %w", err)}
		}

		return atlasMsg{items: buildItems(state)}
	}
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
		data, err := exec.Command("bash", "-c",
			fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, atlasFile)).Output()
		if err != nil {
			data, err = os.ReadFile(getStatePath())
			if err != nil {
				return atlasMsg{err: fmt.Errorf("refresh: %w", err)}
			}
		}
		var state AtlasState
		if err := json.Unmarshal(data, &state); err != nil {
			return atlasMsg{err: fmt.Errorf("parse: %w", err)}
		}
		return atlasMsg{items: buildItems(state)}
	}
}

func buildItems(state AtlasState) []DisplayItem {
	var items []DisplayItem

	pinnedSet := make(map[string]bool)
	for _, p := range state.Pinned {
		pinnedSet[p.Type+"/"+p.Name] = true
	}
	// Active projects are implicitly pinned
	for _, s := range state.Sections {
		if s.Section == "active" {
			for _, e := range s.Entries {
				pinnedSet[e.Type+"/"+e.Name] = true
			}
			break
		}
	}

	for _, section := range state.Sections {
		if len(section.Entries) == 0 {
			continue
		}
		items = append(items, DisplayItem{
			IsHeader:    true,
			SectionName: section.Label,
			EntryCount:  len(section.Entries),
		})
		for _, entry := range section.Entries {
			pinned := pinnedSet[entry.Type+"/"+entry.Name]
			items = append(items, DisplayItem{Entry: entry, IsPinned: pinned})
		}
	}

	return items
}

func checkConnectivity(name, host string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("nc", "-z", "-w", "1", host, "22").Run()
		return connectMsg{name: name, reachable: err == nil}
	}
}

func scheduleTick() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func pinCmd(entryType, name string) tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
		exec.Command("bash", "-c",
			fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && _atlas_pin '%s' '%s'", omx, atlasFile, entryType, name)).Run()
		return nil
	}
}

func unpinCmd(entryType, name string) tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
		exec.Command("bash", "-c",
			fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && _atlas_unpin '%s' '%s'", omx, atlasFile, entryType, name)).Run()
		return nil
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, refreshCmd(), scheduleTick())

	case atlasMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
		} else {
			m.items = msg.items
			m.loadErr = nil
			if m.cursor >= len(m.items) {
				m.cursor = max(0, len(m.items)-1)
			}
			m.snapCursor()
			// Probe remotes
			var cmds []tea.Cmd
			for _, item := range m.items {
				if item.Entry.Type == "remote" && item.Entry.Host != "" {
					cmds = append(cmds, checkConnectivity(item.Entry.Name, item.Entry.Host))
				}
			}
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
		return m, nil

	case connectMsg:
		m.deviceStatus[msg.name] = msg.reachable
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		m.moveDown()

	case "k", "up":
		m.moveUp()

	case "enter":
		if m.cursor < len(m.items) && !m.items[m.cursor].IsHeader {
			item := m.items[m.cursor]
			switch item.Entry.Type {
			case "repo", "dir":
				path := item.Entry.Path
				if path != "" {
					return m, func() tea.Msg {
						_ = exec.Command("tmux", "new-window", "-n", filepath.Base(path), "-c", path).Start()
						return nil
					}
				}
			case "github":
				path := item.Entry.LocalPath
				if path != "" {
					return m, func() tea.Msg {
						_ = exec.Command("tmux", "new-window", "-n", item.Entry.Name, "-c", path).Start()
						return nil
					}
				}
			case "remote":
				if item.Entry.Host != "" && item.Entry.User != "" {
					return m, func() tea.Msg {
						_ = exec.Command("tmux", "new-window", "-n", "ssh-"+item.Entry.Name,
							"ssh", fmt.Sprintf("%s@%s", item.Entry.User, item.Entry.Host)).Start()
						return nil
					}
				}
			}
		}

	case "e":
		if m.cursor < len(m.items) && !m.items[m.cursor].IsHeader {
			item := m.items[m.cursor]
			path := item.Entry.Path
			if path == "" {
				path = item.Entry.LocalPath
			}
			if path != "" {
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", filepath.Base(path), "-c", path).Start()
					return nil
				}
			}
			if item.Entry.Type == "remote" && item.Entry.Host != "" {
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", "ssh-"+item.Entry.Name,
						"ssh", fmt.Sprintf("%s@%s", item.Entry.User, item.Entry.Host)).Start()
					return nil
				}
			}
		}

	case "a":
		if m.cursor < len(m.items) && !m.items[m.cursor].IsHeader {
			item := m.items[m.cursor]
			path := item.Entry.Path
			if path == "" {
				path = item.Entry.LocalPath
			}
			if path != "" {
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", filepath.Base(path)+"-ai", "-c", path,
						"bash", "-ic", "cxx").Start()
					return nil
				}
			}
		}

	case "o":
		if m.cursor < len(m.items) && !m.items[m.cursor].IsHeader {
			item := m.items[m.cursor]
			if item.Entry.Path != "" {
				return m, func() tea.Msg {
					_ = exec.Command("open", "-R", item.Entry.Path).Start()
					return nil
				}
			}
		}

	case "p":
		if m.cursor < len(m.items) && !m.items[m.cursor].IsHeader {
			item := m.items[m.cursor]
			if item.IsPinned {
				return m, unpinCmd(item.Entry.Type, item.Entry.Name)
			}
			return m, pinCmd(item.Entry.Type, item.Entry.Name)
		}

	case "r":
		m.loading = true
		m.loadErr = nil
		return m, tea.Batch(m.spinner.Tick, refreshCmd())
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Cursor helpers
// ---------------------------------------------------------------------------

func (m *model) snapCursor() {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.items[m.cursor].IsHeader {
		m.moveDown()
	}
}

func (m *model) moveDown() {
	start := m.cursor
	for {
		if m.cursor >= len(m.items)-1 {
			m.cursor = start
			return
		}
		m.cursor++
		if !m.items[m.cursor].IsHeader {
			return
		}
	}
}

func (m *model) moveUp() {
	start := m.cursor
	for {
		if m.cursor <= 0 {
			m.cursor = start
			return
		}
		m.cursor--
		if !m.items[m.cursor].IsHeader {
			return
		}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 70
	}

	// Available height for items (minus title + separator + footer)
	viewH := m.height - 4
	if viewH < 5 {
		viewH = 20
	}

	// Title
	title := titleStyle.Render("  Map")
	total := 0
	for _, item := range m.items {
		if !item.IsHeader {
			total++
		}
	}
	countStr := dimStyle.Render(fmt.Sprintf("%d entries", total))
	pad := w - lipgloss.Width(title) - lipgloss.Width(countStr)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(title + strings.Repeat(" ", pad) + countStr + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", max(0, w-4))) + "\n")

	if m.loading && len(m.items) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Loading atlas...\n")
		return b.String()
	}

	if m.loadErr != nil && len(m.items) == 0 {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.loadErr.Error()) + "\n")
		b.WriteString("\n  " + dimStyle.Render("r=retry  q=quit") + "\n")
		return b.String()
	}

	// Scroll: keep cursor in view
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+viewH {
		m.scroll = m.cursor - viewH + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}

	// Render visible items
	end := m.scroll + viewH
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.scroll; i < end; i++ {
		item := m.items[i]
		if item.IsHeader {
			label := item.SectionName
			if item.EntryCount > 0 {
				label += dimStyle.Render(fmt.Sprintf(" (%d)", item.EntryCount))
			}
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  " + sectionStyle.Render(label) + "\n")
			continue
		}

		selected := i == m.cursor
		prefix := "  "
		if selected {
			prefix = "▸ "
		}

		// Pin indicator
		pin := "  "
		if item.IsPinned {
			pin = purpleStyle.Render("★ ")
		}

		switch item.Entry.Type {
		case "repo":
			b.WriteString(m.renderRepo(item, prefix, pin, selected, w))
		case "github":
			b.WriteString(m.renderGithub(item, prefix, pin, selected))
		case "remote":
			b.WriteString(m.renderRemote(item, prefix, pin, selected))
		case "dir":
			b.WriteString(m.renderDir(item, prefix, pin, selected))
		default:
			name := item.Entry.Name
			if len(name) > 20 {
				name = name[:19] + "…"
			}
			b.WriteString("  " + dimStyle.Render(prefix+pin+name) + "\n")
		}
	}

	// Refresh indicator
	if m.loading && len(m.items) > 0 {
		b.WriteString("\n  " + m.spinner.View() + " refreshing...")
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓=nav  ⏎=open  e=shell  a=agent  p=pin  o=finder  r=refresh  q=quit"))
	b.WriteString("\n")

	return b.String()
}

func (m model) renderRepo(item DisplayItem, prefix, pin string, selected bool, w int) string {
	name := item.Entry.Name
	if len(name) > 18 {
		name = name[:17] + "…"
	}
	nameCol := fmt.Sprintf("%-18s", name)

	branch := item.Entry.Branch
	if len(branch) > 8 {
		branch = branch[:7] + "…"
	}

	dirty := ""
	if d := item.Entry.Dirty; d != "" && d != "0" {
		dirty = orangeStyle.Render("Δ")
	}

	if selected {
		return "  " + selectedStyle.Render(prefix) + pin + selectedStyle.Render(nameCol) + " " +
			dimStyle.Render(branch) + " " + dirty + " " + dimStyle.Render(item.Entry.LastCommit) + "\n"
	}
	return "  " + normalStyle.Render(prefix) + pin + normalStyle.Render(nameCol) + " " +
		dimStyle.Render(branch) + " " + dirty + " " + dimStyle.Render(item.Entry.LastCommit) + "\n"
}

func (m model) renderGithub(item DisplayItem, prefix, pin string, selected bool) string {
	name := item.Entry.Name
	if len(name) > 18 {
		name = name[:17] + "…"
	}
	nameCol := fmt.Sprintf("%-18s", name)

	clone := dimStyle.Render("✗")
	if item.Entry.LocalPath != "" {
		clone = greenStyle.Render("✓")
	}

	vis := item.Entry.Visibility

	if selected {
		return "  " + selectedStyle.Render(prefix) + pin + selectedStyle.Render(nameCol) + " " +
			clone + " " + dimStyle.Render(vis) + " " + dimStyle.Render(item.Entry.UpdatedAt) + "\n"
	}
	return "  " + normalStyle.Render(prefix) + pin + normalStyle.Render(nameCol) + " " +
		clone + " " + dimStyle.Render(vis) + " " + dimStyle.Render(item.Entry.UpdatedAt) + "\n"
}

func (m model) renderRemote(item DisplayItem, prefix, pin string, selected bool) string {
	name := item.Entry.Name
	if len(name) > 16 {
		name = name[:15] + "…"
	}
	nameCol := fmt.Sprintf("%-16s", name)

	dot := dimStyle.Render("○")
	if reachable, probed := m.deviceStatus[item.Entry.Name]; probed {
		if reachable {
			dot = greenStyle.Render("●")
		} else {
			dot = errStyle.Render("●")
		}
	}

	desc := item.Entry.Description
	if desc == "" {
		desc = item.Entry.OS
	}

	if selected {
		return "  " + selectedStyle.Render(prefix) + pin + dot + " " + selectedStyle.Render(nameCol) + " " + dimStyle.Render(desc) + "\n"
	}
	return "  " + cyanStyle.Render(prefix) + pin + dot + " " + cyanStyle.Render(nameCol) + " " + dimStyle.Render(desc) + "\n"
}

func (m model) renderDir(item DisplayItem, prefix, pin string, selected bool) string {
	name := item.Entry.Name + "/"
	if len(name) > 20 {
		name = name[:19] + "…"
	}

	if selected {
		return "  " + selectedStyle.Render(prefix) + pin + selectedStyle.Render(name) + "\n"
	}
	return "  " + dimStyle.Render(prefix) + pin + dimStyle.Render(name) + "\n"
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
