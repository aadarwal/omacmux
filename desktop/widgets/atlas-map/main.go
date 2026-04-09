package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	tabActive     = lipgloss.NewStyle().Bold(true).Foreground(bright)
	tabInactive   = lipgloss.NewStyle().Foreground(dim)

	cardDim = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3f3f46")).
		Padding(0, 1)
	cardSel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#e68e0d")).
		Padding(0, 1)

	clusterDim = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3f3f46")).
			Padding(0, 1)
	clusterSel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#5f9ea0")).
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

type AtlasConfig struct {
	Clusters []ClusterDef `json:"clusters"`
}

type ClusterDef struct {
	Name  string   `json:"name"`
	Match []string `json:"match,omitempty"`
	Type  string   `json:"type,omitempty"`
}

// ---------------------------------------------------------------------------
// Computed display types
// ---------------------------------------------------------------------------

type DisplayItem struct {
	IsHeader    bool
	SectionName string
	EntryCount  int
	Entry       AtlasEntry
	IsPinned    bool
}

type CrossRef struct {
	Name      string
	Entry     AtlasEntry
	InActive  bool
	InGithub  bool
	InDesktop bool
	IsPinned  bool
}

type ClusterView struct {
	Name    string
	Entries []AtlasEntry
}

// ---------------------------------------------------------------------------
// View modes
// ---------------------------------------------------------------------------

type ViewMode int

const (
	ModeDashboard ViewMode = iota
	ModeRelations
	ModeGraph
	ModeList
)

var cardDefs = []struct {
	id, label, match string
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
	state  *AtlasState
	config *AtlasConfig
	err    error
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
	config       *AtlasConfig
	spinner      spinner.Model
	loading      bool
	loadErr      error
	deviceStatus map[string]bool
	width, height int

	viewMode ViewMode
	prevMode ViewMode

	// Dashboard
	cardCursor int

	// Relations
	crossRefs []CrossRef
	refCursor int

	// Graph
	clusters    []ClusterView
	graphCursor int // which cluster
	graphItem   int // which item within cluster

	// List drill-down
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

func (m model) sectionEntries(id string) []AtlasEntry {
	if m.state == nil {
		return nil
	}
	var out []AtlasEntry
	for _, s := range m.state.Sections {
		if s.Section == id {
			out = append(out, s.Entries...)
		}
	}
	return out
}

func trunc(s string, n int) string {
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
	switch {
	case strings.HasPrefix(f[1], "second"):
		return f[0] + "s"
	case strings.HasPrefix(f[1], "minute"):
		return f[0] + "m"
	case strings.HasPrefix(f[1], "hour"):
		return f[0] + "h"
	case strings.HasPrefix(f[1], "day"):
		return f[0] + "d"
	case strings.HasPrefix(f[1], "week"):
		return f[0] + "w"
	case strings.HasPrefix(f[1], "month"):
		return f[0] + "mo"
	case strings.HasPrefix(f[1], "year"):
		return f[0] + "y"
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

func renderBar(filled, total, w int) string {
	if total == 0 || w <= 0 {
		return ""
	}
	fw := int(float64(filled) / float64(total) * float64(w))
	if fw > w {
		fw = w
	}
	return greenStyle.Render(strings.Repeat("█", fw)) + dimStyle.Render(strings.Repeat("░", w-fw))
}

func activityBar(commit string, w int) string {
	lv := recencyLevel(commit)
	fw := int(lv * float64(w))
	if fw < 1 && lv > 0 {
		fw = 1
	}
	if fw > w {
		fw = w
	}
	return cyanStyle.Render(strings.Repeat("█", fw)) + dimStyle.Render(strings.Repeat("░", w-fw))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) isPinned(e AtlasEntry) bool {
	if m.state == nil {
		return false
	}
	for _, s := range m.state.Sections {
		if s.Section == "active" {
			for _, ae := range s.Entries {
				if strings.EqualFold(ae.Name, e.Name) {
					return true
				}
			}
		}
	}
	for _, p := range m.state.Pinned {
		if p.Type == e.Type && strings.EqualFold(p.Name, e.Name) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Computed data
// ---------------------------------------------------------------------------

func buildCrossRefs(state *AtlasState) []CrossRef {
	if state == nil {
		return nil
	}
	type info struct {
		entry    AtlasEntry
		active   bool
		github   bool
		desktop  bool
		sections int
	}
	m := make(map[string]*info)

	for _, sec := range state.Sections {
		for _, e := range sec.Entries {
			key := strings.ToLower(e.Name)
			inf, ok := m[key]
			if !ok {
				inf = &info{entry: e}
				m[key] = inf
			}
			switch sec.Section {
			case "active":
				if !inf.active {
					inf.active = true
					inf.sections++
					inf.entry = e // prefer active entry
				}
			case "github":
				if !inf.github {
					inf.github = true
					inf.sections++
				}
			case "desktop":
				if !inf.desktop {
					inf.desktop = true
					inf.sections++
				}
			}
		}
	}

	var refs []CrossRef
	for _, inf := range m {
		if inf.sections < 2 {
			continue
		}
		refs = append(refs, CrossRef{
			Name:      inf.entry.Name,
			Entry:     inf.entry,
			InActive:  inf.active,
			InGithub:  inf.github,
			InDesktop: inf.desktop,
		})
	}
	sort.Slice(refs, func(i, j int) bool {
		si := boolCount(refs[i].InActive, refs[i].InGithub, refs[i].InDesktop)
		sj := boolCount(refs[j].InActive, refs[j].InGithub, refs[j].InDesktop)
		if si != sj {
			return si > sj
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

func boolCount(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

func buildClusters(state *AtlasState, config *AtlasConfig) []ClusterView {
	if state == nil {
		return nil
	}

	// Collect all unique entries
	type entryKey struct{ typ, name string }
	all := make(map[entryKey]AtlasEntry)
	for _, sec := range state.Sections {
		for _, e := range sec.Entries {
			k := entryKey{e.Type, strings.ToLower(e.Name)}
			if _, exists := all[k]; !exists {
				all[k] = e
			}
		}
	}

	claimed := make(map[entryKey]bool)
	var clusters []ClusterView

	if config != nil {
		for _, cd := range config.Clusters {
			var entries []AtlasEntry
			if cd.Type != "" {
				// Match by type
				for k, e := range all {
					if e.Type == cd.Type && !claimed[k] {
						entries = append(entries, e)
						claimed[k] = true
					}
				}
			}
			for _, name := range cd.Match {
				lname := strings.ToLower(name)
				for k, e := range all {
					if strings.ToLower(e.Name) == lname && !claimed[k] {
						entries = append(entries, e)
						claimed[k] = true
					}
				}
			}
			if len(entries) > 0 {
				clusters = append(clusters, ClusterView{Name: cd.Name, Entries: entries})
			}
		}
	}

	// Unclaimed items → "Other"
	var other []AtlasEntry
	for k, e := range all {
		if !claimed[k] {
			other = append(other, e)
		}
	}
	if len(other) > 0 {
		sort.Slice(other, func(i, j int) bool {
			return recencyLevel(other[i].LastCommit) > recencyLevel(other[j].LastCommit)
		})
		clusters = append(clusters, ClusterView{Name: "Other", Entries: other})
	}

	return clusters
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
			af := filepath.Join(omx, "config", "bash", "fns", "atlas")
			data, err = exec.Command("bash", "-c",
				fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, af)).Output()
			if err != nil {
				return atlasMsg{err: fmt.Errorf("atlas: %w", err)}
			}
		} else {
			if info, e := os.Stat(statePath); e == nil && time.Since(info.ModTime()) > 5*time.Minute {
				go func() {
					omx := getOmacmuxPath()
					af := filepath.Join(omx, "config", "bash", "fns", "atlas")
					exec.Command("bash", "-c",
						fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, af)).Run()
				}()
			}
		}
		var state AtlasState
		if err := json.Unmarshal(data, &state); err != nil {
			return atlasMsg{err: fmt.Errorf("parse: %w", err)}
		}
		// Load cluster config
		var cfg AtlasConfig
		cfgPath := filepath.Join(getOmacmuxPath(), "config", "atlas.json")
		if cfgData, err := os.ReadFile(cfgPath); err == nil {
			json.Unmarshal(cfgData, &cfg)
		}
		return atlasMsg{state: &state, config: &cfg}
	}
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		af := filepath.Join(omx, "config", "bash", "fns", "atlas")
		data, err := exec.Command("bash", "-c",
			fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, af)).Output()
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
		var cfg AtlasConfig
		cfgPath := filepath.Join(getOmacmuxPath(), "config", "atlas.json")
		if cfgData, err := os.ReadFile(cfgPath); err == nil {
			json.Unmarshal(cfgData, &cfg)
		}
		return atlasMsg{state: &state, config: &cfg}
	}
}

func checkConn(name, host string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("nc", "-z", "-w", "1", host, "22").Run()
		return connectMsg{name: name, reachable: err == nil}
	}
}

func scheduleTick() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func pinCmd(t, n string) tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		af := filepath.Join(omx, "config", "bash", "fns", "atlas")
		exec.Command("bash", "-c", fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && _atlas_pin '%s' '%s'", omx, af, t, n)).Run()
		return nil
	}
}
func unpinCmd(t, n string) tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		af := filepath.Join(omx, "config", "bash", "fns", "atlas")
		exec.Command("bash", "-c", fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && _atlas_unpin '%s' '%s'", omx, af, t, n)).Run()
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
			m.config = msg.config
			m.loadErr = nil
			m.crossRefs = buildCrossRefs(m.state)
			m.clusters = buildClusters(m.state, m.config)
			var cmds []tea.Cmd
			for _, e := range m.sectionEntries("remote") {
				if e.Host != "" {
					cmds = append(cmds, checkConn(e.Name, e.Host))
				}
			}
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
	case connectMsg:
		m.deviceStatus[msg.name] = msg.reachable
	case tea.KeyMsg:
		switch m.viewMode {
		case ModeDashboard:
			return m.updateDashboard(msg)
		case ModeRelations:
			return m.updateRelations(msg)
		case ModeGraph:
			return m.updateGraph(msg)
		case ModeList:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m model) cycleTab(forward bool) model {
	modes := []ViewMode{ModeDashboard, ModeRelations, ModeGraph}
	cur := 0
	for i, mode := range modes {
		if mode == m.viewMode {
			cur = i
			break
		}
	}
	if forward {
		cur = (cur + 1) % len(modes)
	} else {
		cur = (cur + len(modes) - 1) % len(modes)
	}
	m.viewMode = modes[cur]
	return m
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
		if m.cardCursor >= 2 {
			m.cardCursor -= 2
		}
	case "tab":
		m = m.cycleTab(true)
	case "shift+tab":
		m = m.cycleTab(false)
	case "enter":
		sid := cardDefs[m.cardCursor].match
		m.listItems = buildListForSection(m.state, sid)
		m.listTitle = cardDefs[m.cardCursor].label
		m.listCursor = 0
		m.listScroll = 0
		m.prevMode = ModeDashboard
		m.viewMode = ModeList
	case "r":
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, refreshCmd())
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Relations
// ---------------------------------------------------------------------------

func (m model) updateRelations(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.refCursor < len(m.crossRefs)-1 {
			m.refCursor++
		}
	case "k", "up":
		if m.refCursor > 0 {
			m.refCursor--
		}
	case "tab":
		m = m.cycleTab(true)
	case "shift+tab":
		m = m.cycleTab(false)
	case "enter", "e":
		if m.refCursor < len(m.crossRefs) {
			ref := m.crossRefs[m.refCursor]
			path := ref.Entry.Path
			if path == "" {
				path = ref.Entry.LocalPath
			}
			if path != "" {
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", filepath.Base(path), "-c", path).Start()
					return nil
				}
			}
		}
	case "p":
		if m.refCursor < len(m.crossRefs) {
			ref := m.crossRefs[m.refCursor]
			if m.isPinned(ref.Entry) {
				return m, unpinCmd(ref.Entry.Type, ref.Entry.Name)
			}
			return m, pinCmd(ref.Entry.Type, ref.Entry.Name)
		}
	case "r":
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, refreshCmd())
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Graph
// ---------------------------------------------------------------------------

func (m model) updateGraph(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.graphCursor < len(m.clusters) {
			cl := m.clusters[m.graphCursor]
			if m.graphItem < len(cl.Entries)-1 {
				m.graphItem++
			} else if m.graphCursor+2 < len(m.clusters) {
				m.graphCursor += 2
				m.graphItem = 0
			}
		}
	case "k", "up":
		if m.graphItem > 0 {
			m.graphItem--
		} else if m.graphCursor >= 2 {
			m.graphCursor -= 2
			if m.graphCursor < len(m.clusters) {
				m.graphItem = max(0, len(m.clusters[m.graphCursor].Entries)-1)
			}
		}
	case "l", "right":
		if m.graphCursor%2 == 0 && m.graphCursor+1 < len(m.clusters) {
			m.graphCursor++
			m.graphItem = 0
		}
	case "h", "left":
		if m.graphCursor%2 == 1 {
			m.graphCursor--
			m.graphItem = 0
		}
	case "tab":
		m = m.cycleTab(true)
	case "shift+tab":
		m = m.cycleTab(false)
	case "enter", "e":
		if m.graphCursor < len(m.clusters) {
			cl := m.clusters[m.graphCursor]
			if m.graphItem < len(cl.Entries) {
				e := cl.Entries[m.graphItem]
				path := e.Path
				if path == "" {
					path = e.LocalPath
				}
				if path != "" {
					return m, func() tea.Msg {
						_ = exec.Command("tmux", "new-window", "-n", filepath.Base(path), "-c", path).Start()
						return nil
					}
				}
				if e.Type == "remote" && e.Host != "" && e.User != "" {
					return m, func() tea.Msg {
						_ = exec.Command("tmux", "new-window", "-n", "ssh-"+e.Name,
							"ssh", fmt.Sprintf("%s@%s", e.User, e.Host)).Start()
						return nil
					}
				}
			}
		}
	case "r":
		m.loading = true
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
		m.viewMode = m.prevMode
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
			path := m.listItems[m.listCursor].Entry.Path
			if path == "" {
				path = m.listItems[m.listCursor].Entry.LocalPath
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

func buildListForSection(state *AtlasState, sectionID string) []DisplayItem {
	if state == nil {
		return nil
	}
	var items []DisplayItem
	for _, sec := range state.Sections {
		if sec.Section != sectionID {
			continue
		}
		for _, e := range sec.Entries {
			items = append(items, DisplayItem{Entry: e})
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	switch m.viewMode {
	case ModeDashboard:
		return m.viewDashboard()
	case ModeRelations:
		return m.viewRelations()
	case ModeGraph:
		return m.viewGraph()
	case ModeList:
		return m.viewList()
	}
	return ""
}

func (m model) renderTabs() string {
	tabs := []struct {
		name string
		mode ViewMode
	}{
		{"Cards", ModeDashboard},
		{"Relations", ModeRelations},
		{"Graph", ModeGraph},
	}
	var parts []string
	for _, t := range tabs {
		if t.mode == m.viewMode {
			parts = append(parts, tabActive.Render("["+t.name+"]"))
		} else {
			parts = append(parts, tabInactive.Render(" "+t.name+" "))
		}
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// View — Dashboard (Cards)
// ---------------------------------------------------------------------------

func (m model) viewDashboard() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 100
	}

	b.WriteString("  " + titleStyle.Render("Map") + "  " + m.renderTabs() + "\n")

	if m.loading && m.state == nil {
		b.WriteString("\n  " + m.spinner.View() + " Loading atlas...\n")
		return b.String()
	}
	if m.loadErr != nil && m.state == nil {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.loadErr.Error()) + "\n")
		return b.String()
	}

	cw := (w - 5) / 2
	if cw < 28 {
		cw = 28
	}

	b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, m.dashCard(0, cw, 6), " ", m.dashCard(1, cw, 6)) + "\n")
	b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, m.dashCard(2, cw, 3), " ", m.dashCard(3, cw, 3)) + "\n")
	b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, m.dashCard(4, cw, 1), " ", m.dashCard(5, cw, 1)) + "\n")

	if m.loading {
		b.WriteString("  " + m.spinner.View() + " refreshing...\n")
	}
	sel := cardDefs[m.cardCursor].label
	b.WriteString(footerStyle.Render(fmt.Sprintf("  ▸ %s  ←→↑↓=nav  ⏎=expand  Tab=view  r=refresh  q=quit", sel)) + "\n")
	return b.String()
}

func (m model) dashCard(idx, cw, maxL int) string {
	if idx >= len(cardDefs) {
		return ""
	}
	def := cardDefs[idx]
	entries := m.sectionEntries(def.match)
	sel := idx == m.cardCursor
	st := cardDim.Width(cw - 2)
	if sel {
		st = cardSel.Width(cw - 2)
	}
	iw := cw - 6
	hdr := sectionStyle.Render(def.label) + " " + dimStyle.Render(fmt.Sprintf("(%d)", len(entries)))
	var body string
	switch def.id {
	case "active":
		body = m.cActive(entries, maxL, iw)
	case "remote":
		body = m.cRemote(entries, maxL, iw)
	case "github":
		body = m.cGithub(entries, maxL, iw)
	case "desktop":
		body = m.cDesktop(entries, maxL, iw)
	case "icloud":
		body = m.cIcloud(entries, iw)
	case "drives":
		body = m.cDrives(iw)
	}
	if body == "" {
		body = dimStyle.Render("empty")
	}
	return st.Render(hdr + "\n" + body)
}

func (m model) cActive(es []AtlasEntry, maxL, iw int) string {
	if len(es) == 0 {
		return dimStyle.Render("no projects")
	}
	show := maxL
	more := false
	if len(es) > show {
		show = maxL - 1
		more = true
	} else {
		show = len(es)
	}
	var lines []string
	for i := 0; i < show; i++ {
		e := es[i]
		d := "  "
		if e.Dirty != "" && e.Dirty != "0" {
			d = orangeStyle.Render("Δ ")
		}
		lines = append(lines, fmt.Sprintf("%-12s%s%-7s %s %s",
			trunc(e.Name, 12), d, trunc(e.Branch, 7), activityBar(e.LastCommit, 8), dimStyle.Render(shortAgo(e.LastCommit))))
	}
	if more {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("+%d more", len(es)-show)))
	}
	return strings.Join(lines, "\n")
}

func (m model) cRemote(es []AtlasEntry, maxL, iw int) string {
	if len(es) == 0 {
		return dimStyle.Render("no hosts")
	}
	show := maxL
	more := false
	if len(es) > show {
		show = maxL - 1
		more = true
	} else {
		show = len(es)
	}
	var lines []string
	for i := 0; i < show; i++ {
		e := es[i]
		dot := dimStyle.Render("○")
		if r, ok := m.deviceStatus[e.Name]; ok {
			if r {
				dot = greenStyle.Render("●")
			} else {
				dot = errStyle.Render("●")
			}
		}
		desc := e.Description
		if desc == "" {
			desc = e.OS
		}
		lines = append(lines, dot+" "+cyanStyle.Render(fmt.Sprintf("%-15s", trunc(e.Name, 15)))+" "+dimStyle.Render(trunc(desc, iw-20)))
	}
	if more {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("+%d more", len(es)-show)))
	}
	return strings.Join(lines, "\n")
}

func (m model) cGithub(es []AtlasEntry, maxL, iw int) string {
	if len(es) == 0 {
		return dimStyle.Render("gh unavailable")
	}
	cl := 0
	for _, e := range es {
		if e.LocalPath != "" {
			cl++
		}
	}
	bw := max(iw-18, 6)
	l1 := renderBar(cl, len(es), bw) + " " + greenStyle.Render(fmt.Sprintf("%d", cl)) + dimStyle.Render(" cloned")
	l2 := dimStyle.Render(fmt.Sprintf("%d remote only", len(es)-cl))
	var top []string
	for i := 0; i < len(es) && len(top) < 3; i++ {
		top = append(top, trunc(es[i].Name, 11))
	}
	l3 := dimStyle.Render("recent: ") + normalStyle.Render(strings.Join(top, " "))
	lines := []string{l1, l2}
	if maxL >= 3 {
		lines = append(lines, l3)
	}
	return strings.Join(lines, "\n")
}

func (m model) cDesktop(es []AtlasEntry, maxL, iw int) string {
	if len(es) == 0 {
		return dimStyle.Render("empty")
	}
	repos, dirty := 0, 0
	for _, e := range es {
		if e.Type == "repo" {
			repos++
			if d, err := strconv.Atoi(e.Dirty); err == nil && d > 0 {
				dirty++
			} else if e.Dirty == "+" || e.Dirty == "1" {
				dirty++
			}
		}
	}
	l1 := normalStyle.Render(fmt.Sprintf("%d repos", repos)) + dimStyle.Render(fmt.Sprintf(" · %d other", len(es)-repos))
	bw := max(iw-20, 4)
	l2 := renderBar(dirty, repos, bw) + " " + orangeStyle.Render(fmt.Sprintf("%d dirty", dirty)) + dimStyle.Render(fmt.Sprintf(" · %d clean", repos-dirty))
	var top []string
	for i := 0; i < len(es) && len(top) < 3; i++ {
		if es[i].Type == "repo" {
			top = append(top, trunc(es[i].Name, 11))
		}
	}
	l3 := dimStyle.Render("recent: ") + normalStyle.Render(strings.Join(top, " "))
	lines := []string{l1, l2}
	if maxL >= 3 {
		lines = append(lines, l3)
	}
	return strings.Join(lines, "\n")
}

func (m model) cIcloud(es []AtlasEntry, iw int) string {
	if len(es) == 0 {
		return dimStyle.Render("not available")
	}
	var ns []string
	for _, e := range es {
		ns = append(ns, trunc(e.Name, 11)+"/")
	}
	return dimStyle.Render(strings.Join(ns, "  "))
}

func (m model) cDrives(iw int) string {
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
	return dimStyle.Render("no drives")
}

// ---------------------------------------------------------------------------
// View — Relations
// ---------------------------------------------------------------------------

func (m model) viewRelations() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 100
	}

	b.WriteString("  " + titleStyle.Render("Map") + "  " + m.renderTabs() + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", max(0, w-4))) + "\n")

	if m.state == nil {
		b.WriteString("\n  " + m.spinner.View() + " Loading...\n")
		return b.String()
	}

	// Cross-reference matrix
	b.WriteString("  " + sectionStyle.Render("Cross-References") + " " + dimStyle.Render(fmt.Sprintf("(%d entities in 2+ sections)", len(m.crossRefs))) + "\n")
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("  %-20s  Active  GitHub  Desktop  %s", "", "Activity")) + "\n")

	viewH := m.height - 10
	if viewH < 5 {
		viewH = 15
	}
	show := len(m.crossRefs)
	if show > viewH {
		show = viewH
	}

	for i := 0; i < show; i++ {
		ref := m.crossRefs[i]
		sel := i == m.refCursor
		pre := "  "
		if sel {
			pre = "▸ "
		}

		name := fmt.Sprintf("%-20s", trunc(ref.Name, 20))
		active := dimStyle.Render("  ·   ")
		if ref.InActive {
			active = purpleStyle.Render("  ★   ")
		}
		gh := dimStyle.Render("  ·   ")
		if ref.InGithub {
			gh = greenStyle.Render("  ✓   ")
		}
		desk := dimStyle.Render("  ·   ")
		if ref.InDesktop {
			desk = greenStyle.Render("  ✓     ")
		}

		ago := ref.Entry.LastCommit
		if ago == "" {
			ago = ref.Entry.UpdatedAt
		}
		bar := activityBar(ago, 12)
		agoS := shortAgo(ago)

		if sel {
			b.WriteString("  " + selectedStyle.Render(pre+name) + active + gh + desk + bar + " " + dimStyle.Render(agoS) + "\n")
		} else {
			b.WriteString("  " + normalStyle.Render(pre+name) + active + gh + desk + bar + " " + dimStyle.Render(agoS) + "\n")
		}
	}

	if len(m.crossRefs) > viewH {
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("  +%d more", len(m.crossRefs)-viewH)) + "\n")
	}

	if m.loading {
		b.WriteString("\n  " + m.spinner.View() + " refreshing...")
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓=nav  ⏎=open  p=pin  Tab=view  r=refresh  q=quit") + "\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// View — Graph (Cluster Map)
// ---------------------------------------------------------------------------

func (m model) viewGraph() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 100
	}

	b.WriteString("  " + titleStyle.Render("Map") + "  " + m.renderTabs() + "\n")

	if m.state == nil || len(m.clusters) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Loading...\n")
		return b.String()
	}

	cw := (w - 5) / 2
	if cw < 28 {
		cw = 28
	}

	// Render clusters in 2-column grid
	for row := 0; row*2 < len(m.clusters); row++ {
		leftIdx := row * 2
		rightIdx := row*2 + 1

		left := m.renderCluster(leftIdx, cw)
		right := ""
		if rightIdx < len(m.clusters) {
			right = m.renderCluster(rightIdx, cw)
		}

		if right != "" {
			b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n")
		} else {
			b.WriteString("  " + left + "\n")
		}
	}

	if m.loading {
		b.WriteString("  " + m.spinner.View() + " refreshing...\n")
	}

	// Show selected item info
	selInfo := ""
	if m.graphCursor < len(m.clusters) {
		cl := m.clusters[m.graphCursor]
		selInfo = cl.Name
		if m.graphItem < len(cl.Entries) {
			selInfo += " · " + cl.Entries[m.graphItem].Name
		}
	}
	b.WriteString(footerStyle.Render(fmt.Sprintf("  ▸ %s", selInfo)) + "\n")
	b.WriteString(footerStyle.Render("  ←→↑↓=nav  ⏎=open  Tab=view  r=refresh  q=quit") + "\n")
	return b.String()
}

func (m model) renderCluster(idx, cw int) string {
	if idx >= len(m.clusters) {
		return ""
	}
	cl := m.clusters[idx]
	isSel := idx == m.graphCursor

	st := clusterDim.Width(cw - 2)
	if isSel {
		st = clusterSel.Width(cw - 2)
	}
	iw := cw - 6

	hdr := sectionStyle.Render(cl.Name) + " " + dimStyle.Render(fmt.Sprintf("(%d)", len(cl.Entries)))

	maxShow := 7
	show := len(cl.Entries)
	more := false
	if show > maxShow {
		show = maxShow - 1
		more = true
	}

	var lines []string
	for i := 0; i < show; i++ {
		e := cl.Entries[i]
		itemSel := isSel && i == m.graphItem
		pre := "  "
		if itemSel {
			pre = "▸ "
		}

		var line string
		switch e.Type {
		case "remote":
			dot := dimStyle.Render("○")
			if r, ok := m.deviceStatus[e.Name]; ok {
				if r {
					dot = greenStyle.Render("●")
				} else {
					dot = errStyle.Render("●")
				}
			}
			desc := e.Description
			if desc == "" {
				desc = e.OS
			}
			nm := cyanStyle.Render(fmt.Sprintf("%-14s", trunc(e.Name, 14)))
			if itemSel {
				nm = selectedStyle.Render(fmt.Sprintf("%-14s", trunc(e.Name, 14)))
			}
			line = pre + dot + " " + nm + " " + dimStyle.Render(trunc(desc, iw-20))

		default:
			nm := trunc(e.Name, 14)
			dirty := "  "
			if e.Dirty != "" && e.Dirty != "0" {
				dirty = orangeStyle.Render("Δ ")
			}
			ago := e.LastCommit
			if ago == "" {
				ago = e.UpdatedAt
			}
			bar := activityBar(ago, 6)
			agoS := shortAgo(ago)
			if itemSel {
				line = selectedStyle.Render(pre+fmt.Sprintf("%-14s", nm)) + dirty + bar + " " + dimStyle.Render(agoS)
			} else {
				line = normalStyle.Render(pre+fmt.Sprintf("%-14s", nm)) + dirty + bar + " " + dimStyle.Render(agoS)
			}
		}
		lines = append(lines, line)
	}
	if more {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  +%d more", len(cl.Entries)-show)))
	}

	return st.Render(hdr + "\n" + strings.Join(lines, "\n"))
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

	crumb := dimStyle.Render("  Map") + dimStyle.Render(" > ") + titleStyle.Render(m.listTitle)
	cnt := dimStyle.Render(fmt.Sprintf("%d entries", len(m.listItems)))
	pad := w - lipgloss.Width(crumb) - lipgloss.Width(cnt)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(crumb + strings.Repeat(" ", pad) + cnt + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", max(0, w-4))) + "\n")

	if len(m.listItems) == 0 {
		b.WriteString("\n  " + dimStyle.Render("empty") + "\n")
		b.WriteString("\n" + footerStyle.Render("  Esc=back  q=quit") + "\n")
		return b.String()
	}

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
		sel := i == m.listCursor
		pre := "  "
		if sel {
			pre = "▸ "
		}
		pin := "  "
		if m.isPinned(item.Entry) {
			pin = purpleStyle.Render("★ ")
		}

		switch item.Entry.Type {
		case "repo":
			nm := fmt.Sprintf("%-20s", trunc(item.Entry.Name, 20))
			d := "  "
			if item.Entry.Dirty != "" && item.Entry.Dirty != "0" {
				d = orangeStyle.Render("Δ ")
			}
			bar := activityBar(item.Entry.LastCommit, 8)
			ago := shortAgo(item.Entry.LastCommit)
			if sel {
				b.WriteString("  " + selectedStyle.Render(pre) + pin + selectedStyle.Render(nm) + d +
					dimStyle.Render(fmt.Sprintf("%-10s", trunc(item.Entry.Branch, 10))) + " " + bar + " " + dimStyle.Render(ago) + "\n")
			} else {
				b.WriteString("  " + normalStyle.Render(pre) + pin + normalStyle.Render(nm) + d +
					dimStyle.Render(fmt.Sprintf("%-10s", trunc(item.Entry.Branch, 10))) + " " + bar + " " + dimStyle.Render(ago) + "\n")
			}
		case "github":
			nm := fmt.Sprintf("%-20s", trunc(item.Entry.Name, 20))
			cl := dimStyle.Render("✗")
			if item.Entry.LocalPath != "" {
				cl = greenStyle.Render("✓")
			}
			if sel {
				b.WriteString("  " + selectedStyle.Render(pre) + pin + selectedStyle.Render(nm) + " " + cl + " " +
					dimStyle.Render(item.Entry.Visibility) + " " + dimStyle.Render(item.Entry.UpdatedAt) + "\n")
			} else {
				b.WriteString("  " + normalStyle.Render(pre) + pin + normalStyle.Render(nm) + " " + cl + " " +
					dimStyle.Render(item.Entry.Visibility) + " " + dimStyle.Render(item.Entry.UpdatedAt) + "\n")
			}
		case "remote":
			nm := fmt.Sprintf("%-18s", trunc(item.Entry.Name, 18))
			dot := dimStyle.Render("○")
			if r, ok := m.deviceStatus[item.Entry.Name]; ok {
				if r {
					dot = greenStyle.Render("●")
				} else {
					dot = errStyle.Render("●")
				}
			}
			desc := item.Entry.Description
			if desc == "" {
				desc = item.Entry.OS
			}
			if sel {
				b.WriteString("  " + selectedStyle.Render(pre) + pin + dot + " " + selectedStyle.Render(nm) + " " + dimStyle.Render(desc) + "\n")
			} else {
				b.WriteString("  " + cyanStyle.Render(pre) + pin + dot + " " + cyanStyle.Render(nm) + " " + dimStyle.Render(desc) + "\n")
			}
		default:
			nm := trunc(item.Entry.Name, 24)
			if sel {
				b.WriteString("  " + selectedStyle.Render(pre) + pin + selectedStyle.Render(nm) + "\n")
			} else {
				b.WriteString("  " + dimStyle.Render(pre) + pin + dimStyle.Render(nm) + "\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓=nav  ⏎/e=open  a=agent  p=pin  Esc=back  q=quit") + "\n")
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
