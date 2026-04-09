package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	cardDim = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3f3f46")).
		Padding(0, 1)

	cardSelected = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#e68e0d")).
			Padding(0, 1)
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
	EntryCount  int
	Entry       AtlasEntry
	IsPinned    bool
}

// ---------------------------------------------------------------------------
// View modes
// ---------------------------------------------------------------------------

type ViewMode int

const (
	ModeDashboard ViewMode = iota
	ModeList
)

// Card definitions: which atlas sections map to which cards
var cardDefs = []struct {
	id    string
	label string
	match string // section name to match
}{
	{"active", "ACTIVE PROJECTS", "active"},
	{"remote", "REMOTE", "remote"},
	{"github", "GITHUB", "github"},
	{"desktop", "DESKTOP", "desktop"},
	{"icloud", "ICLOUD", "icloud"},
	{"drives", "DRIVES", "drive"},
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type tickMsg time.Time
type atlasMsg struct {
	state *AtlasState
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
	state        *AtlasState
	spinner      spinner.Model
	loading      bool
	loadErr      error
	deviceStatus map[string]bool
	width        int
	height       int

	// Dashboard mode
	viewMode   ViewMode
	cardCursor int // 0-5

	// List mode
	listItems  []DisplayItem
	listCursor int
	listScroll int
	listTitle  string
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
// Helpers
// ---------------------------------------------------------------------------

func (m model) sectionEntries(sectionID string) []AtlasEntry {
	if m.state == nil {
		return nil
	}
	var entries []AtlasEntry
	for _, s := range m.state.Sections {
		if s.Section == sectionID {
			entries = append(entries, s.Entries...)
		}
	}
	return entries
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func shortAgo(s string) string {
	s = strings.TrimSuffix(s, " ago")
	f := strings.Fields(s)
	if len(f) < 2 {
		return s
	}
	n := f[0]
	switch {
	case strings.HasPrefix(f[1], "second"):
		return n + "s"
	case strings.HasPrefix(f[1], "minute"):
		return n + "m"
	case strings.HasPrefix(f[1], "hour"):
		return n + "h"
	case strings.HasPrefix(f[1], "day"):
		return n + "d"
	case strings.HasPrefix(f[1], "week"):
		return n + "w"
	case strings.HasPrefix(f[1], "month"):
		return n + "mo"
	case strings.HasPrefix(f[1], "year"):
		return n + "y"
	}
	return s
}

func recencyLevel(s string) float64 {
	f := strings.Fields(strings.TrimSuffix(s, " ago"))
	if len(f) < 2 {
		return 0.05
	}
	switch {
	case strings.HasPrefix(f[1], "second"), strings.HasPrefix(f[1], "minute"):
		return 1.0
	case strings.HasPrefix(f[1], "hour"):
		return 0.85
	case strings.HasPrefix(f[1], "day"):
		return 0.6
	case strings.HasPrefix(f[1], "week"):
		return 0.35
	case strings.HasPrefix(f[1], "month"):
		return 0.15
	}
	return 0.05
}

func renderBar(filled, total, width int) string {
	if total == 0 || width <= 0 {
		return ""
	}
	ratio := float64(filled) / float64(total)
	fw := int(ratio * float64(width))
	if fw < 0 {
		fw = 0
	}
	if fw > width {
		fw = width
	}
	return greenStyle.Render(strings.Repeat("█", fw)) + dimStyle.Render(strings.Repeat("░", width-fw))
}

func activityBar(lastCommit string, width int) string {
	level := recencyLevel(lastCommit)
	fw := int(level * float64(width))
	if fw < 1 && level > 0 {
		fw = 1
	}
	if fw > width {
		fw = width
	}
	return cyanStyle.Render(strings.Repeat("█", fw)) + dimStyle.Render(strings.Repeat("░", width-fw))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
			omx := getOmacmuxPath()
			atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
			data, err = exec.Command("bash", "-c",
				fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, atlasFile)).Output()
			if err != nil {
				return atlasMsg{err: fmt.Errorf("atlas: %w", err)}
			}
		} else {
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
		return atlasMsg{state: &state}
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
		return atlasMsg{state: &state}
	}
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
// List building (for drill-down)
// ---------------------------------------------------------------------------

func buildListItems(state *AtlasState, sectionID string) []DisplayItem {
	if state == nil {
		return nil
	}
	pinnedSet := make(map[string]bool)
	for _, p := range state.Pinned {
		pinnedSet[p.Type+"/"+p.Name] = true
	}
	for _, s := range state.Sections {
		if s.Section == "active" {
			for _, e := range s.Entries {
				pinnedSet[e.Type+"/"+e.Name] = true
			}
		}
	}

	var items []DisplayItem
	for _, section := range state.Sections {
		if section.Section != sectionID {
			continue
		}
		for _, entry := range section.Entries {
			items = append(items, DisplayItem{
				Entry:    entry,
				IsPinned: pinnedSet[entry.Type+"/"+entry.Name],
			})
		}
	}
	return items
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
			m.state = msg.state
			m.loadErr = nil
			// Probe remotes
			var cmds []tea.Cmd
			for _, e := range m.sectionEntries("remote") {
				if e.Host != "" {
					cmds = append(cmds, checkConnectivity(e.Name, e.Host))
				}
			}
			// If in list mode, rebuild list items
			if m.viewMode == ModeList {
				sid := cardDefs[m.cardCursor].match
				m.listItems = buildListItems(m.state, sid)
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
		switch m.viewMode {
		case ModeDashboard:
			return m.updateDashboard(msg)
		case ModeList:
			return m.updateList(msg)
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Dashboard
// ---------------------------------------------------------------------------

func (m model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "l", "right":
		if m.cardCursor%2 == 0 && m.cardCursor+1 < len(cardDefs) {
			m.cardCursor++
		}
	case "h", "left":
		if m.cardCursor%2 == 1 {
			m.cardCursor--
		}
	case "j", "down":
		if m.cardCursor+2 < len(cardDefs) {
			m.cardCursor += 2
		}
	case "k", "up":
		if m.cardCursor-2 >= 0 {
			m.cardCursor -= 2
		}
	case "tab":
		m.cardCursor = (m.cardCursor + 1) % len(cardDefs)
	case "shift+tab":
		m.cardCursor = (m.cardCursor + len(cardDefs) - 1) % len(cardDefs)

	case "enter":
		sid := cardDefs[m.cardCursor].match
		m.listItems = buildListItems(m.state, sid)
		m.listTitle = cardDefs[m.cardCursor].label
		m.listCursor = 0
		m.listScroll = 0
		m.viewMode = ModeList

	case "r":
		m.loading = true
		m.loadErr = nil
		return m, tea.Batch(m.spinner.Tick, refreshCmd())
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — List
// ---------------------------------------------------------------------------

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "backspace":
		m.viewMode = ModeDashboard
		return m, nil

	case "j", "down":
		if m.listCursor < len(m.listItems)-1 {
			m.listCursor++
		}
	case "k", "up":
		if m.listCursor > 0 {
			m.listCursor--
		}

	case "enter", "e":
		if m.listCursor < len(m.listItems) {
			item := m.listItems[m.listCursor]
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
			if item.Entry.Type == "remote" && item.Entry.Host != "" && item.Entry.User != "" {
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", "ssh-"+item.Entry.Name,
						"ssh", fmt.Sprintf("%s@%s", item.Entry.User, item.Entry.Host)).Start()
					return nil
				}
			}
		}

	case "a":
		if m.listCursor < len(m.listItems) {
			item := m.listItems[m.listCursor]
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

	case "p":
		if m.listCursor < len(m.listItems) {
			item := m.listItems[m.listCursor]
			if item.IsPinned {
				return m, unpinCmd(item.Entry.Type, item.Entry.Name)
			}
			return m, pinCmd(item.Entry.Type, item.Entry.Name)
		}

	case "r":
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, refreshCmd())
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	switch m.viewMode {
	case ModeDashboard:
		return m.viewDashboard()
	case ModeList:
		return m.viewList()
	}
	return ""
}

// ---------------------------------------------------------------------------
// View — Dashboard
// ---------------------------------------------------------------------------

func (m model) viewDashboard() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 100
	}

	// Title
	title := titleStyle.Render("  Map")
	total := 0
	if m.state != nil {
		for _, s := range m.state.Sections {
			total += len(s.Entries)
		}
	}
	countStr := dimStyle.Render(fmt.Sprintf("%d entries · %d sections", total, len(cardDefs)))
	pad := w - lipgloss.Width(title) - lipgloss.Width(countStr)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(title + strings.Repeat(" ", pad) + countStr + "\n")

	if m.loading && m.state == nil {
		b.WriteString("\n  " + m.spinner.View() + " Loading atlas...\n")
		return b.String()
	}
	if m.loadErr != nil && m.state == nil {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.loadErr.Error()) + "\n")
		b.WriteString("\n  " + dimStyle.Render("r=retry  q=quit") + "\n")
		return b.String()
	}

	cardW := (w - 5) / 2 // 2 cols, 1 gap, 2 margin
	if cardW < 30 {
		cardW = 30
	}

	// Row 0: Active Projects + Remote (detailed, more lines)
	row0 := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderCard(0, cardW, 6),
		" ",
		m.renderCard(1, cardW, 6),
	)
	b.WriteString("  " + row0 + "\n")

	// Row 1: GitHub + Desktop (summary, fewer lines)
	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderCard(2, cardW, 3),
		" ",
		m.renderCard(3, cardW, 3),
	)
	b.WriteString("  " + row1 + "\n")

	// Row 2: iCloud + Drives (compact)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderCard(4, cardW, 1),
		" ",
		m.renderCard(5, cardW, 1),
	)
	b.WriteString("  " + row2 + "\n")

	// Refresh indicator
	if m.loading && m.state != nil {
		b.WriteString("  " + m.spinner.View() + " refreshing...\n")
	}

	// Footer with selected card name
	selName := cardDefs[m.cardCursor].label
	b.WriteString(footerStyle.Render(fmt.Sprintf("  ▸ %s", selName)) + "\n")
	b.WriteString(footerStyle.Render("  ←→↑↓/hjkl=nav  Tab=cycle  ⏎=expand  r=refresh  q=quit") + "\n")

	return b.String()
}

func (m model) renderCard(cardIdx, cardWidth, maxLines int) string {
	if cardIdx >= len(cardDefs) {
		return ""
	}
	def := cardDefs[cardIdx]
	entries := m.sectionEntries(def.match)
	count := len(entries)
	selected := cardIdx == m.cardCursor

	style := cardDim.Width(cardWidth - 2)
	if selected {
		style = cardSelected.Width(cardWidth - 2)
	}

	contentW := cardWidth - 6 // borders + padding

	// Header
	header := sectionStyle.Render(def.label) + " " + dimStyle.Render(fmt.Sprintf("(%d)", count))

	// Body
	var body string
	switch def.id {
	case "active":
		body = m.cardActive(entries, maxLines, contentW)
	case "remote":
		body = m.cardRemote(entries, maxLines, contentW)
	case "github":
		body = m.cardGithub(entries, maxLines, contentW)
	case "desktop":
		body = m.cardDesktop(entries, maxLines, contentW)
	case "icloud":
		body = m.cardIcloud(entries, contentW)
	case "drives":
		body = m.cardDrives(contentW)
	}

	if body == "" {
		body = dimStyle.Render("empty")
	}

	return style.Render(header + "\n" + body)
}

func (m model) cardActive(entries []AtlasEntry, maxLines, contentW int) string {
	if len(entries) == 0 {
		return dimStyle.Render("no projects")
	}
	nameW := 13
	barW := 8
	show := maxLines
	if len(entries) > show {
		show = maxLines - 1 // reserve for "+N more"
	} else {
		show = len(entries)
	}

	var lines []string
	for i := 0; i < show; i++ {
		e := entries[i]
		name := truncate(e.Name, nameW)

		dirty := "  "
		if d := e.Dirty; d != "" && d != "0" {
			dirty = orangeStyle.Render("Δ ")
		}

		branch := truncate(e.Branch, 7)
		bar := activityBar(e.LastCommit, barW)
		ago := shortAgo(e.LastCommit)

		line := fmt.Sprintf("%-*s%s%-7s %s %s",
			nameW, name, dirty, branch, bar, dimStyle.Render(ago))
		lines = append(lines, line)
	}
	if len(entries) > maxLines {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("+%d more", len(entries)-show)))
	}
	return strings.Join(lines, "\n")
}

func (m model) cardRemote(entries []AtlasEntry, maxLines, contentW int) string {
	if len(entries) == 0 {
		return dimStyle.Render("no hosts")
	}
	nameW := 16
	show := maxLines
	if len(entries) > show {
		show = maxLines - 1
	} else {
		show = len(entries)
	}

	var lines []string
	for i := 0; i < show; i++ {
		e := entries[i]
		name := truncate(e.Name, nameW)

		dot := dimStyle.Render("○")
		if reachable, probed := m.deviceStatus[e.Name]; probed {
			if reachable {
				dot = greenStyle.Render("●")
			} else {
				dot = errStyle.Render("●")
			}
		}

		desc := e.Description
		if desc == "" {
			desc = e.OS
		}
		desc = truncate(desc, contentW-nameW-4)

		line := dot + " " + cyanStyle.Render(fmt.Sprintf("%-*s", nameW, name)) + " " + dimStyle.Render(desc)
		lines = append(lines, line)
	}
	if len(entries) > maxLines {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("+%d more", len(entries)-show)))
	}
	return strings.Join(lines, "\n")
}

func (m model) cardGithub(entries []AtlasEntry, maxLines, contentW int) string {
	if len(entries) == 0 {
		return dimStyle.Render("gh not available")
	}
	total := len(entries)
	cloned := 0
	for _, e := range entries {
		if e.LocalPath != "" {
			cloned++
		}
	}

	barW := max(contentW-20, 8)
	bar := renderBar(cloned, total, barW)

	line1 := bar + " " + greenStyle.Render(fmt.Sprintf("%d", cloned)) + dimStyle.Render(" cloned")
	line2 := dimStyle.Render(fmt.Sprintf("%d remote only", total-cloned))

	// Top recent repos
	var top []string
	for i := 0; i < len(entries) && len(top) < 3; i++ {
		top = append(top, truncate(entries[i].Name, 12))
	}
	line3 := dimStyle.Render("recent: ") + normalStyle.Render(strings.Join(top, " "))

	lines := []string{line1, line2}
	if maxLines >= 3 {
		lines = append(lines, line3)
	}
	return strings.Join(lines, "\n")
}

func (m model) cardDesktop(entries []AtlasEntry, maxLines, contentW int) string {
	if len(entries) == 0 {
		return dimStyle.Render("empty")
	}
	repos := 0
	dirty := 0
	for _, e := range entries {
		if e.Type == "repo" {
			repos++
			if d, err := strconv.Atoi(e.Dirty); err == nil && d > 0 {
				dirty++
			} else if e.Dirty == "+" || e.Dirty == "1" {
				dirty++
			}
		}
	}
	other := len(entries) - repos
	clean := repos - dirty

	line1 := normalStyle.Render(fmt.Sprintf("%d repos", repos)) + dimStyle.Render(fmt.Sprintf(" · %d other", other))

	barW := max(contentW-22, 6)
	bar := renderBar(dirty, repos, barW)
	line2 := bar + " " + orangeStyle.Render(fmt.Sprintf("%d dirty", dirty)) + dimStyle.Render(fmt.Sprintf(" · %d clean", clean))

	var top []string
	for i := 0; i < len(entries) && len(top) < 3; i++ {
		if entries[i].Type == "repo" {
			top = append(top, truncate(entries[i].Name, 12))
		}
	}
	line3 := dimStyle.Render("recent: ") + normalStyle.Render(strings.Join(top, " "))

	lines := []string{line1, line2}
	if maxLines >= 3 {
		lines = append(lines, line3)
	}
	return strings.Join(lines, "\n")
}

func (m model) cardIcloud(entries []AtlasEntry, contentW int) string {
	if len(entries) == 0 {
		return dimStyle.Render("not available")
	}
	var names []string
	for _, e := range entries {
		names = append(names, truncate(e.Name, 12)+"/")
	}
	return dimStyle.Render(strings.Join(names, "  "))
}

func (m model) cardDrives(contentW int) string {
	entries := m.sectionEntries("drive")
	if len(entries) == 0 {
		// Check for drive sections with label
		if m.state != nil {
			for _, s := range m.state.Sections {
				if s.Section == "drive" {
					if len(s.Entries) == 0 {
						return dimStyle.Render(s.Label + " — not mounted")
					}
					return normalStyle.Render(s.Label) + dimStyle.Render(fmt.Sprintf(" · %d items", len(s.Entries)))
				}
			}
		}
		return dimStyle.Render("no drives configured")
	}
	return normalStyle.Render(fmt.Sprintf("%d items", len(entries)))
}

// ---------------------------------------------------------------------------
// View — List (drill-down)
// ---------------------------------------------------------------------------

func (m model) viewList() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 80
	}

	viewH := m.height - 5
	if viewH < 5 {
		viewH = 20
	}

	// Breadcrumb
	crumb := dimStyle.Render("  Map") + dimStyle.Render(" > ") + titleStyle.Render(m.listTitle)
	countStr := dimStyle.Render(fmt.Sprintf("%d entries", len(m.listItems)))
	pad := w - lipgloss.Width(crumb) - lipgloss.Width(countStr)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(crumb + strings.Repeat(" ", pad) + countStr + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", max(0, w-4))) + "\n")

	if len(m.listItems) == 0 {
		b.WriteString("\n  " + dimStyle.Render("empty") + "\n")
		b.WriteString("\n" + footerStyle.Render("  Esc=back  q=quit") + "\n")
		return b.String()
	}

	// Scroll
	if m.listCursor < m.listScroll {
		m.listScroll = m.listCursor
	}
	if m.listCursor >= m.listScroll+viewH {
		m.listScroll = m.listCursor - viewH + 1
	}

	end := m.listScroll + viewH
	if end > len(m.listItems) {
		end = len(m.listItems)
	}

	for i := m.listScroll; i < end; i++ {
		item := m.listItems[i]
		selected := i == m.listCursor
		prefix := "  "
		if selected {
			prefix = "▸ "
		}

		pin := "  "
		if item.IsPinned {
			pin = purpleStyle.Render("★ ")
		}

		switch item.Entry.Type {
		case "repo":
			b.WriteString(m.listRenderRepo(item, prefix, pin, selected, w))
		case "github":
			b.WriteString(m.listRenderGithub(item, prefix, pin, selected, w))
		case "remote":
			b.WriteString(m.listRenderRemote(item, prefix, pin, selected))
		case "dir":
			name := item.Entry.Name + "/"
			if len(name) > 24 {
				name = name[:23] + "…"
			}
			if selected {
				b.WriteString("  " + selectedStyle.Render(prefix) + pin + selectedStyle.Render(name) + "\n")
			} else {
				b.WriteString("  " + dimStyle.Render(prefix) + pin + dimStyle.Render(name) + "\n")
			}
		default:
			name := truncate(item.Entry.Name, 24)
			b.WriteString("  " + dimStyle.Render(prefix+pin+name) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓=nav  ⏎/e=open  a=agent  p=pin  Esc=back  q=quit") + "\n")

	return b.String()
}

func (m model) listRenderRepo(item DisplayItem, prefix, pin string, selected bool, w int) string {
	name := truncate(item.Entry.Name, 20)
	nameCol := fmt.Sprintf("%-20s", name)
	dirty := "  "
	if d := item.Entry.Dirty; d != "" && d != "0" {
		dirty = orangeStyle.Render("Δ ")
	}
	branch := truncate(item.Entry.Branch, 10)
	bar := activityBar(item.Entry.LastCommit, 8)
	ago := shortAgo(item.Entry.LastCommit)

	if selected {
		return "  " + selectedStyle.Render(prefix) + pin + selectedStyle.Render(nameCol) + dirty +
			dimStyle.Render(fmt.Sprintf("%-10s", branch)) + " " + bar + " " + dimStyle.Render(ago) + "\n"
	}
	return "  " + normalStyle.Render(prefix) + pin + normalStyle.Render(nameCol) + dirty +
		dimStyle.Render(fmt.Sprintf("%-10s", branch)) + " " + bar + " " + dimStyle.Render(ago) + "\n"
}

func (m model) listRenderGithub(item DisplayItem, prefix, pin string, selected bool, w int) string {
	name := truncate(item.Entry.Name, 20)
	nameCol := fmt.Sprintf("%-20s", name)
	clone := dimStyle.Render("✗")
	if item.Entry.LocalPath != "" {
		clone = greenStyle.Render("✓")
	}
	vis := item.Entry.Visibility
	date := item.Entry.UpdatedAt

	if selected {
		return "  " + selectedStyle.Render(prefix) + pin + selectedStyle.Render(nameCol) + " " +
			clone + " " + dimStyle.Render(vis) + " " + dimStyle.Render(date) + "\n"
	}
	return "  " + normalStyle.Render(prefix) + pin + normalStyle.Render(nameCol) + " " +
		clone + " " + dimStyle.Render(vis) + " " + dimStyle.Render(date) + "\n"
}

func (m model) listRenderRemote(item DisplayItem, prefix, pin string, selected bool) string {
	name := truncate(item.Entry.Name, 18)
	nameCol := fmt.Sprintf("%-18s", name)
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
