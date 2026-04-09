package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	selectedStyle = lipgloss.NewStyle().Foreground(bright).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(white)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	footerStyle   = lipgloss.NewStyle().Foreground(dim)
	greenStyle    = lipgloss.NewStyle().Foreground(green)
	cyanStyle     = lipgloss.NewStyle().Foreground(cyan)
	orangeStyle   = lipgloss.NewStyle().Foreground(orange)
	errStyle      = lipgloss.NewStyle().Foreground(red)
	sectionStyle  = lipgloss.NewStyle().Foreground(orange).Bold(true)
	breadStyle    = lipgloss.NewStyle().Foreground(dim)
)

// ---------------------------------------------------------------------------
// View levels
// ---------------------------------------------------------------------------

type ViewLevel int

const (
	LevelList         ViewLevel = iota // atlas overview
	LevelRepoDetail                    // repo PRs/branches
	LevelPRDetail                      // single PR
	LevelRemoteDetail                  // remote host info
)

// ---------------------------------------------------------------------------
// Atlas data types
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

// DisplayItem is a flattened item for rendering in Level 0.
type DisplayItem struct {
	IsHeader    bool
	SectionName string
	Entry       AtlasEntry
	PRCount     int
}

// ---------------------------------------------------------------------------
// PR/Branch types (for repo drill-down)
// ---------------------------------------------------------------------------

type PR struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	HeadRefName    string `json:"headRefName"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt      string `json:"createdAt"`
	ReviewDecision string `json:"reviewDecision"`
	IsDraft        bool   `json:"isDraft"`
}

type Branch struct {
	Name string `json:"name"`
}

type PRDetail struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	HeadRefName    string `json:"headRefName"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	ChangedFiles   int    `json:"changedFiles"`
	ReviewDecision string `json:"reviewDecision"`
	URL            string `json:"url"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt         string `json:"createdAt"`
	StatusCheckRollup []struct {
		Context    string `json:"context"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

// ---------------------------------------------------------------------------
// System status types
// ---------------------------------------------------------------------------

type SwarmFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Topology string `json:"topology"`
	Agents   []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"agents"`
}

// ---------------------------------------------------------------------------
// Tea messages
// ---------------------------------------------------------------------------

type tickMsg time.Time

type atlasStateMsg struct {
	items []DisplayItem
	err   error
}

type ownerMsg struct {
	owner string
}

type prCountsMsg struct {
	counts map[string]int
}

type systemInfoMsg struct {
	sessions    int
	swarmCount  int
	agentTotal  int
	agentActive int
}

type connectMsg struct {
	name      string
	reachable bool
}

type repoDetailMsg struct {
	prs      []PR
	branches []Branch
	err      error
}

type prDetailMsg struct {
	detail *PRDetail
	err    error
}

// ---------------------------------------------------------------------------
// Detail sections
// ---------------------------------------------------------------------------

type DetailSection int

const (
	SectionPRs DetailSection = iota
	SectionBranches
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	owner   string
	spinner spinner.Model
	width   int
	height  int

	// Navigation
	viewLevel ViewLevel

	// Level 0 — Atlas list
	items   []DisplayItem
	cursor  int
	loading bool
	loadErr error

	// System status (absorbed from status widget)
	sessions     int
	swarmCount   int
	agentTotal   int
	agentActive  int
	deviceStatus map[string]bool // device name → reachable

	// Level 1 — Repo detail
	activeItem    *DisplayItem
	prs           []PR
	branches      []Branch
	detailSection DetailSection
	detailCursor  int
	detailLoading bool
	detailErr     error

	// Level 2 — PR detail
	activePR  *PR
	prDetail  *PRDetail
	prLoading bool
	prErr     error
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
	return tea.Batch(m.spinner.Tick, loadAtlasStateCmd(), fetchOwnerCmd(), fetchSystemInfoCmd(), scheduleTickCmd())
}

// ---------------------------------------------------------------------------
// Helpers — item accessors
// ---------------------------------------------------------------------------

func (m model) itemLocalPath() string {
	if m.activeItem == nil {
		return ""
	}
	if m.activeItem.Entry.Path != "" {
		return m.activeItem.Entry.Path
	}
	return m.activeItem.Entry.LocalPath
}

func (m model) itemIsGithub() bool {
	if m.activeItem == nil {
		return false
	}
	return m.activeItem.Entry.HasGithub == "true" || m.activeItem.Entry.Type == "github"
}

func (m model) itemRepoURL() string {
	if m.owner == "" || m.activeItem == nil {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s", m.owner, m.activeItem.Entry.Name)
}

// Snap cursor to nearest selectable item (skip headers).
func (m *model) snapCursorToSelectable() {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.items[m.cursor].IsHeader {
		m.moveCursorDown()
	}
}

func (m *model) moveCursorDown() {
	start := m.cursor
	for {
		if m.cursor >= len(m.items)-1 {
			m.cursor = start // don't move past end
			return
		}
		m.cursor++
		if !m.items[m.cursor].IsHeader {
			return
		}
	}
}

func (m *model) moveCursorUp() {
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
// Commands — Atlas state
// ---------------------------------------------------------------------------

func getAtlasStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "omacmux", "atlas", "state.json")
}

func getOmacmuxPath() string {
	if p := os.Getenv("OMACMUX_PATH"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "omacmux")
}

func loadAtlasStateCmd() tea.Cmd {
	return func() tea.Msg {
		statePath := getAtlasStatePath()

		// Try cached state first
		data, err := os.ReadFile(statePath)
		if err != nil || len(data) == 0 {
			// No cache — generate fresh
			data, err = refreshAtlasState()
			if err != nil {
				return atlasStateMsg{err: fmt.Errorf("atlas: %w", err)}
			}
		} else {
			// If stale (>5 min), trigger background refresh but use stale data now
			if info, serr := os.Stat(statePath); serr == nil {
				if time.Since(info.ModTime()) > 5*time.Minute {
					go refreshAtlasState()
				}
			}
		}

		var state AtlasState
		if err := json.Unmarshal(data, &state); err != nil {
			return atlasStateMsg{err: fmt.Errorf("parse state.json: %w", err)}
		}

		return atlasStateMsg{items: buildDisplayItems(state)}
	}
}

func refreshAndLoadCmd() tea.Cmd {
	return func() tea.Msg {
		data, err := refreshAtlasState()
		if err != nil {
			// Fall back to cached file
			data, err = os.ReadFile(getAtlasStatePath())
			if err != nil {
				return atlasStateMsg{err: fmt.Errorf("atlas refresh: %w", err)}
			}
		}

		var state AtlasState
		if err := json.Unmarshal(data, &state); err != nil {
			return atlasStateMsg{err: fmt.Errorf("parse: %w", err)}
		}

		return atlasStateMsg{items: buildDisplayItems(state)}
	}
}

func refreshAtlasState() ([]byte, error) {
	omx := getOmacmuxPath()
	atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf("export OMACMUX_PATH='%s' && source '%s' && map --json 2>/dev/null", omx, atlasFile))
	return cmd.Output()
}

func buildDisplayItems(state AtlasState) []DisplayItem {
	var items []DisplayItem

	// Build pinned lookup: type/name → true
	pinnedSet := make(map[string]bool)
	for _, p := range state.Pinned {
		pinnedSet[p.Type+"/"+p.Name] = true
	}

	for _, section := range state.Sections {
		var sectionItems []DisplayItem

		for _, entry := range section.Entries {
			if section.Section == "active" {
				// Active projects: always shown
				sectionItems = append(sectionItems, DisplayItem{Entry: entry})
			} else if pinnedSet[entry.Type+"/"+entry.Name] {
				// Other sections: only show pinned items
				sectionItems = append(sectionItems, DisplayItem{Entry: entry})
			}
		}

		if len(sectionItems) > 0 {
			items = append(items, DisplayItem{
				IsHeader:    true,
				SectionName: section.Label,
			})
			items = append(items, sectionItems...)
		}
	}

	return items
}

func fetchOwnerCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("gh", "api", "user", "--jq", ".login").Output()
		if err != nil {
			return ownerMsg{}
		}
		return ownerMsg{owner: strings.TrimSpace(string(out))}
	}
}

func fetchPRCountsCmd(owner string, items []DisplayItem) tea.Cmd {
	return func() tea.Msg {
		counts := make(map[string]int)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, item := range items {
			if item.IsHeader {
				continue
			}
			isGH := item.Entry.HasGithub == "true" || item.Entry.Type == "github"
			if !isGH {
				continue
			}
			name := item.Entry.Name
			wg.Add(1)
			go func(repoName string) {
				defer wg.Done()
				fullName := owner + "/" + repoName
				out, err := exec.Command("gh", "pr", "list",
					"--repo", fullName,
					"--state", "open",
					"--json", "number",
					"--limit", "100",
				).Output()
				if err != nil {
					return
				}
				var prs []struct{ Number int }
				if json.Unmarshal(out, &prs) == nil {
					mu.Lock()
					counts[repoName] = len(prs)
					mu.Unlock()
				}
			}(name)
		}
		wg.Wait()

		return prCountsMsg{counts: counts}
	}
}

func scheduleTickCmd() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Commands — System status (absorbed from status widget)
// ---------------------------------------------------------------------------

func fetchSystemInfoCmd() tea.Cmd {
	return func() tea.Msg {
		var info systemInfoMsg

		// Tmux sessions
		out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line != "" {
					info.sessions++
				}
			}
		}

		// Swarms
		home, _ := os.UserHomeDir()
		swarmGlob := filepath.Join(home, ".local", "share", "omacmux", "swarms", "*", "swarm.json")
		matches, _ := filepath.Glob(swarmGlob)
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var sf SwarmFile
			if json.Unmarshal(data, &sf) != nil {
				continue
			}
			info.swarmCount++
			info.agentTotal += len(sf.Agents)
			for _, a := range sf.Agents {
				if a.Status == "active" {
					info.agentActive++
				}
			}
		}

		return info
	}
}

func checkConnectivityCmd(name, host string, port int) tea.Cmd {
	return func() tea.Msg {
		if port == 0 {
			port = 22
		}
		err := exec.Command("nc", "-z", "-w", "1", host, fmt.Sprintf("%d", port)).Run()
		return connectMsg{name: name, reachable: err == nil}
	}
}

// ---------------------------------------------------------------------------
// Commands — Level 1 (repo detail)
// ---------------------------------------------------------------------------

func fetchRepoDetailCmd(owner, repoName string) tea.Cmd {
	return func() tea.Msg {
		fullName := owner + "/" + repoName
		var msg repoDetailMsg
		var wg sync.WaitGroup

		// Fetch PRs
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := exec.Command("gh", "pr", "list",
				"--repo", fullName,
				"--state", "open",
				"--json", "number,title,author,createdAt,headRefName,reviewDecision,isDraft",
				"--limit", "20",
			).Output()
			if err != nil {
				return
			}
			json.Unmarshal(out, &msg.prs)
		}()

		// Fetch branches
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := exec.Command("gh", "api",
				fmt.Sprintf("repos/%s/branches", fullName),
				"--jq", ".[].name",
				"-q", ".[:10]",
			).Output()
			if err != nil {
				out2, err2 := exec.Command("gh", "api",
					fmt.Sprintf("repos/%s/branches?per_page=10", fullName),
				).Output()
				if err2 != nil {
					return
				}
				json.Unmarshal(out2, &msg.branches)
				return
			}
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line != "" {
					msg.branches = append(msg.branches, Branch{Name: line})
				}
			}
		}()

		wg.Wait()
		return msg
	}
}

func fetchLocalBranchesCmd(localPath string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("git", "-C", localPath, "branch", "--format=%(refname:short)").Output()
		if err != nil {
			return repoDetailMsg{}
		}
		var branches []Branch
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				branches = append(branches, Branch{Name: line})
			}
		}
		return repoDetailMsg{branches: branches}
	}
}

// ---------------------------------------------------------------------------
// Commands — Level 2 (PR detail)
// ---------------------------------------------------------------------------

func fetchPRDetailCmd(owner, repoName string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		fullName := owner + "/" + repoName
		out, err := exec.Command("gh", "pr", "view",
			fmt.Sprintf("%d", prNumber),
			"--repo", fullName,
			"--json", "number,title,body,headRefName,additions,deletions,changedFiles,reviewDecision,url,author,createdAt,statusCheckRollup",
		).Output()
		if err != nil {
			return prDetailMsg{err: fmt.Errorf("gh pr view: %w", err)}
		}
		var detail PRDetail
		if err := json.Unmarshal(out, &detail); err != nil {
			return prDetailMsg{err: fmt.Errorf("JSON parse: %w", err)}
		}
		return prDetailMsg{detail: &detail}
	}
}

// ---------------------------------------------------------------------------
// Commands — Tmux actions
// ---------------------------------------------------------------------------

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", url).Start()
		return nil
	}
}

func shellInPathCmd(localPath string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("tmux", "new-window", "-n", filepath.Base(localPath), "-c", localPath).Start()
		return nil
	}
}

func agentInPathCmd(localPath string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("tmux", "new-window", "-n", filepath.Base(localPath)+"-ai", "-c", localPath,
			"bash", "-ic", "cxx").Start()
		return nil
	}
}

func scanRepoCmd(localPath string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("tmux", "new-window", "-n", filepath.Base(localPath)+"-scan", "-c", localPath,
			"bash", "-ic", "scan explore").Start()
		return nil
	}
}

func cloneAndOpenCmd(owner, repoName string) tea.Cmd {
	return func() tea.Msg {
		home, _ := os.UserHomeDir()
		dest := filepath.Join(home, repoName)
		_ = exec.Command("tmux", "new-window", "-n", repoName,
			"bash", "-ic", fmt.Sprintf("gh repo clone %s/%s %s && cd %s && exec bash", owner, repoName, dest, dest)).Start()
		return nil
	}
}

func checkoutPRCmd(localPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		name := fmt.Sprintf("pr-%d", prNumber)
		_ = exec.Command("tmux", "new-window", "-n", name, "-c", localPath,
			"bash", "-ic", fmt.Sprintf("gh pr checkout %d && exec bash", prNumber)).Start()
		return nil
	}
}

func agentOnPRCmd(localPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		name := fmt.Sprintf("pr-%d-ai", prNumber)
		_ = exec.Command("tmux", "new-window", "-n", name, "-c", localPath,
			"bash", "-ic", fmt.Sprintf("gh pr checkout %d && cxx", prNumber)).Start()
		return nil
	}
}

func launchAtlasCmd() tea.Cmd {
	return func() tea.Msg {
		omx := getOmacmuxPath()
		atlasFile := filepath.Join(omx, "config", "bash", "fns", "atlas")
		_ = exec.Command("tmux", "new-window", "-n", "atlas",
			"bash", "-ic", fmt.Sprintf("source '%s' && map", atlasFile)).Start()
		return nil
	}
}

func sshRemoteCmd(name, host, user string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("tmux", "new-window", "-n", "ssh-"+name,
			"ssh", fmt.Sprintf("%s@%s", user, host)).Start()
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
		m.loadErr = nil
		return m, tea.Batch(m.spinner.Tick, refreshAndLoadCmd(), fetchSystemInfoCmd(), scheduleTickCmd())

	case atlasStateMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
		} else {
			m.items = msg.items
			m.loadErr = nil
			if m.cursor >= len(m.items) {
				m.cursor = maxInt(0, len(m.items)-1)
			}
			m.snapCursorToSelectable()
			// Fire connectivity probes for remote entries
			var cmds []tea.Cmd
			for _, item := range m.items {
				if item.Entry.Type == "remote" && item.Entry.Host != "" {
					cmds = append(cmds, checkConnectivityCmd(item.Entry.Name, item.Entry.Host, 22))
				}
			}
			// Fetch PR counts if owner is known
			if m.owner != "" {
				cmds = append(cmds, fetchPRCountsCmd(m.owner, m.items))
			}
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
		return m, nil

	case ownerMsg:
		if msg.owner != "" {
			m.owner = msg.owner
			if len(m.items) > 0 {
				return m, fetchPRCountsCmd(m.owner, m.items)
			}
		}
		return m, nil

	case systemInfoMsg:
		m.sessions = msg.sessions
		m.swarmCount = msg.swarmCount
		m.agentTotal = msg.agentTotal
		m.agentActive = msg.agentActive
		return m, nil

	case connectMsg:
		m.deviceStatus[msg.name] = msg.reachable
		return m, nil

	case prCountsMsg:
		for i := range m.items {
			if count, ok := msg.counts[m.items[i].Entry.Name]; ok {
				m.items[i].PRCount = count
			}
		}
		return m, nil

	case repoDetailMsg:
		m.detailLoading = false
		if msg.err != nil {
			m.detailErr = msg.err
		} else {
			m.prs = msg.prs
			m.branches = msg.branches
			m.detailErr = nil
		}
		return m, nil

	case prDetailMsg:
		m.prLoading = false
		if msg.err != nil {
			m.prErr = msg.err
		} else {
			m.prDetail = msg.detail
			m.prErr = nil
		}
		return m, nil

	case tea.KeyMsg:
		switch m.viewLevel {
		case LevelList:
			return m.updateList(msg)
		case LevelRepoDetail:
			return m.updateRepoDetail(msg)
		case LevelPRDetail:
			return m.updatePRDetail(msg)
		case LevelRemoteDetail:
			return m.updateRemoteDetail(msg)
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Level 0 (atlas list)
// ---------------------------------------------------------------------------

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		m.moveCursorDown()

	case "k", "up":
		m.moveCursorUp()

	case "enter":
		if m.cursor >= len(m.items) || m.items[m.cursor].IsHeader {
			break
		}
		item := m.items[m.cursor]
		switch item.Entry.Type {
		case "repo":
			m.activeItem = &item
			m.viewLevel = LevelRepoDetail
			m.detailLoading = true
			m.detailCursor = 0
			m.detailSection = SectionPRs
			m.prs = nil
			m.branches = nil
			m.detailErr = nil
			cmds := []tea.Cmd{m.spinner.Tick}
			if m.owner != "" && item.Entry.HasGithub == "true" {
				cmds = append(cmds, fetchRepoDetailCmd(m.owner, item.Entry.Name))
			} else if item.Entry.Path != "" {
				cmds = append(cmds, fetchLocalBranchesCmd(item.Entry.Path))
			}
			return m, tea.Batch(cmds...)

		case "github":
			m.activeItem = &item
			m.viewLevel = LevelRepoDetail
			m.detailLoading = true
			m.detailCursor = 0
			m.detailSection = SectionPRs
			m.prs = nil
			m.branches = nil
			m.detailErr = nil
			cmds := []tea.Cmd{m.spinner.Tick}
			if m.owner != "" {
				cmds = append(cmds, fetchRepoDetailCmd(m.owner, item.Entry.Name))
			}
			return m, tea.Batch(cmds...)

		case "remote":
			m.activeItem = &item
			m.viewLevel = LevelRemoteDetail
			return m, nil
		}

	case "o":
		if m.cursor < len(m.items) && !m.items[m.cursor].IsHeader {
			item := m.items[m.cursor]
			switch item.Entry.Type {
			case "repo":
				if item.Entry.HasGithub == "true" && m.owner != "" {
					return m, openBrowserCmd(fmt.Sprintf("https://github.com/%s/%s", m.owner, item.Entry.Name))
				}
			case "github":
				if m.owner != "" {
					return m, openBrowserCmd(fmt.Sprintf("https://github.com/%s/%s", m.owner, item.Entry.Name))
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
				return m, shellInPathCmd(path)
			} else if item.Entry.Type == "github" && m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, item.Entry.Name)
			} else if item.Entry.Type == "remote" && item.Entry.Host != "" {
				return m, sshRemoteCmd(item.Entry.Name, item.Entry.Host, item.Entry.User)
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
				return m, agentInPathCmd(path)
			} else if item.Entry.Type == "github" && m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, item.Entry.Name)
			}
		}

	case "m":
		return m, launchAtlasCmd()

	case "r":
		m.loading = true
		m.loadErr = nil
		return m, tea.Batch(m.spinner.Tick, refreshAndLoadCmd())
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Level 1 (repo detail)
// ---------------------------------------------------------------------------

func (m model) updateRepoDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "backspace":
		m.viewLevel = LevelList
		m.activeItem = nil
		return m, nil

	case "j", "down":
		m.detailCursor++
		m.clampDetailCursor()

	case "k", "up":
		if m.detailCursor > 0 {
			m.detailCursor--
		}

	case "tab":
		if m.detailSection == SectionPRs && len(m.branches) > 0 {
			m.detailSection = SectionBranches
		} else {
			m.detailSection = SectionPRs
		}
		m.detailCursor = 0

	case "shift+tab":
		if m.detailSection == SectionBranches {
			m.detailSection = SectionPRs
		} else if len(m.branches) > 0 {
			m.detailSection = SectionBranches
		}
		m.detailCursor = 0

	case "enter":
		if m.detailSection == SectionPRs && m.detailCursor < len(m.prs) {
			pr := m.prs[m.detailCursor]
			m.activePR = &pr
			m.viewLevel = LevelPRDetail
			m.prLoading = true
			m.prDetail = nil
			m.prErr = nil
			if m.owner != "" && m.activeItem != nil {
				return m, tea.Batch(m.spinner.Tick, fetchPRDetailCmd(m.owner, m.activeItem.Entry.Name, pr.Number))
			}
		}
		if m.detailSection == SectionBranches && m.detailCursor < len(m.branches) {
			branch := m.branches[m.detailCursor]
			localPath := m.itemLocalPath()
			if localPath != "" {
				name := filepath.Base(localPath) + "/" + branch.Name
				if len(name) > 20 {
					name = branch.Name
				}
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", name, "-c", localPath,
						"bash", "-ic", fmt.Sprintf("git checkout %s 2>/dev/null || git checkout -b %s && exec bash", branch.Name, branch.Name)).Start()
					return nil
				}
			}
		}

	case "e":
		if localPath := m.itemLocalPath(); localPath != "" {
			return m, shellInPathCmd(localPath)
		} else if m.owner != "" && m.activeItem != nil {
			return m, cloneAndOpenCmd(m.owner, m.activeItem.Entry.Name)
		}

	case "a":
		if localPath := m.itemLocalPath(); localPath != "" {
			return m, agentInPathCmd(localPath)
		} else if m.owner != "" && m.activeItem != nil {
			return m, cloneAndOpenCmd(m.owner, m.activeItem.Entry.Name)
		}

	case "s":
		if localPath := m.itemLocalPath(); localPath != "" {
			return m, scanRepoCmd(localPath)
		}

	case "o":
		if url := m.itemRepoURL(); url != "" {
			return m, openBrowserCmd(url)
		}

	case "r":
		if m.activeItem != nil && m.owner != "" && m.itemIsGithub() {
			m.detailLoading = true
			m.detailErr = nil
			return m, tea.Batch(m.spinner.Tick, fetchRepoDetailCmd(m.owner, m.activeItem.Entry.Name))
		}
	}

	return m, nil
}

func (m *model) clampDetailCursor() {
	var maxIdx int
	switch m.detailSection {
	case SectionPRs:
		maxIdx = len(m.prs) - 1
	case SectionBranches:
		maxIdx = len(m.branches) - 1
	}
	if maxIdx < 0 {
		maxIdx = 0
	}
	if m.detailCursor > maxIdx {
		m.detailCursor = maxIdx
	}
}

// ---------------------------------------------------------------------------
// Update — Level 2 (PR detail)
// ---------------------------------------------------------------------------

func (m model) updatePRDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "backspace":
		m.viewLevel = LevelRepoDetail
		m.activePR = nil
		m.prDetail = nil
		return m, nil

	case "e":
		if m.activeItem != nil && m.activePR != nil {
			if localPath := m.itemLocalPath(); localPath != "" {
				return m, checkoutPRCmd(localPath, m.activePR.Number)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, m.activeItem.Entry.Name)
			}
		}

	case "a":
		if m.activeItem != nil && m.activePR != nil {
			if localPath := m.itemLocalPath(); localPath != "" {
				return m, agentOnPRCmd(localPath, m.activePR.Number)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, m.activeItem.Entry.Name)
			}
		}

	case "o":
		if m.prDetail != nil && m.prDetail.URL != "" {
			return m, openBrowserCmd(m.prDetail.URL)
		} else if url := m.itemRepoURL(); url != "" {
			return m, openBrowserCmd(url)
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Remote detail
// ---------------------------------------------------------------------------

func (m model) updateRemoteDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "backspace":
		m.viewLevel = LevelList
		m.activeItem = nil
		return m, nil

	case "e":
		if m.activeItem != nil {
			e := m.activeItem.Entry
			if e.Host != "" && e.User != "" {
				return m, sshRemoteCmd(e.Name, e.Host, e.User)
			}
		}

	case "a":
		if m.activeItem != nil {
			e := m.activeItem.Entry
			if e.Host != "" && e.User != "" {
				return m, func() tea.Msg {
					_ = exec.Command("tmux", "new-window", "-n", "ssh-"+e.Name+"-ai",
						"bash", "-ic", fmt.Sprintf("ssh %s@%s -t 'claude'", e.User, e.Host)).Start()
					return nil
				}
			}
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	switch m.viewLevel {
	case LevelList:
		return m.viewList()
	case LevelRepoDetail:
		return m.viewRepoDetail()
	case LevelPRDetail:
		return m.viewPRDetail()
	case LevelRemoteDetail:
		return m.viewRemoteDetail()
	}
	return ""
}

// ---------------------------------------------------------------------------
// View — Level 0 (atlas list)
// ---------------------------------------------------------------------------

func (m model) viewList() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	// Title row
	title := titleStyle.Render("  Atlas")
	count := 0
	for _, item := range m.items {
		if !item.IsHeader {
			count++
		}
	}
	countStr := ""
	if count > 0 {
		countStr = dimStyle.Render(fmt.Sprintf("%d pinned", count))
	}
	titlePad := w - lipgloss.Width(title) - lipgloss.Width(countStr)
	if titlePad < 1 {
		titlePad = 1
	}
	b.WriteString(title + strings.Repeat(" ", titlePad) + countStr + "\n")

	// System status bar
	var statusParts []string
	if m.sessions > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d sessions", m.sessions))
	}
	if m.swarmCount > 0 {
		s := fmt.Sprintf("%d swarm", m.swarmCount)
		if m.swarmCount > 1 {
			s += "s"
		}
		if m.agentTotal > 0 {
			s += fmt.Sprintf(" (%d agents)", m.agentActive)
		}
		statusParts = append(statusParts, s)
	}
	if len(statusParts) > 0 {
		b.WriteString("  " + dimStyle.Render(strings.Join(statusParts, " · ")) + "\n")
	}

	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", maxInt(0, w-4))) + "\n")

	// Loading (no items yet)
	if m.loading && len(m.items) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Loading atlas...\n")
		return b.String()
	}

	// Error (no items)
	if m.loadErr != nil && len(m.items) == 0 {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.loadErr.Error()) + "\n")
		b.WriteString("\n  " + dimStyle.Render("Press r to retry, q to quit") + "\n")
		return b.String()
	}

	// Render items
	for i, item := range m.items {
		if item.IsHeader {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  " + sectionStyle.Render(item.SectionName) + "\n")
			continue
		}

		selected := i == m.cursor
		prefix := "  "
		if selected {
			prefix = "▸ "
		}

		switch item.Entry.Type {
		case "repo":
			b.WriteString(m.renderRepoItem(item, prefix, selected, w))
		case "github":
			b.WriteString(m.renderGithubItem(item, prefix, selected, w))
		case "remote":
			b.WriteString(m.renderRemoteItem(item, prefix, selected))
		case "dir":
			b.WriteString(m.renderDirItem(item, prefix, selected))
		}
	}

	// Refresh indicator
	if m.loading && len(m.items) > 0 {
		b.WriteString("\n  " + m.spinner.View() + " refreshing...")
	}
	if m.loadErr != nil && len(m.items) > 0 {
		b.WriteString("\n  " + errStyle.Render("refresh failed: "+m.loadErr.Error()))
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓/jk=nav  ⏎=detail  m=map  o=web  e=shell  a=agent  r=refresh  q=quit"))
	b.WriteString("\n")

	return b.String()
}

func (m model) renderRepoItem(item DisplayItem, prefix string, selected bool, w int) string {
	name := item.Entry.Name
	if len(name) > 20 {
		name = name[:19] + "…"
	}
	nameCol := fmt.Sprintf("%-20s", name)

	branch := item.Entry.Branch
	if len(branch) > 10 {
		branch = branch[:9] + "…"
	}
	branchCol := fmt.Sprintf("%-10s", branch)

	// Dirty indicator
	dirty := ""
	if d, err := strconv.Atoi(item.Entry.Dirty); err == nil && d > 0 {
		dirty = orangeStyle.Render(fmt.Sprintf("%dΔ", d))
	}

	// PR count
	prStr := ""
	if item.PRCount == 1 {
		prStr = "1 PR"
	} else if item.PRCount > 1 {
		prStr = fmt.Sprintf("%d PRs", item.PRCount)
	}

	// Last commit
	ago := item.Entry.LastCommit

	var line string
	if selected {
		line = "  " + selectedStyle.Render(prefix+nameCol) + " " + dimStyle.Render(branchCol)
	} else {
		line = "  " + normalStyle.Render(prefix+nameCol) + " " + dimStyle.Render(branchCol)
	}
	if dirty != "" {
		line += " " + dirty
	}
	if prStr != "" {
		line += "  " + orangeStyle.Render(prStr)
	}
	if ago != "" {
		line += "  " + dimStyle.Render(ago)
	}
	return line + "\n"
}

func (m model) renderGithubItem(item DisplayItem, prefix string, selected bool, w int) string {
	name := item.Entry.Name
	if len(name) > 20 {
		name = name[:19] + "…"
	}
	nameCol := fmt.Sprintf("%-20s", name)

	cloneInd := dimStyle.Render("✗")
	if item.Entry.LocalPath != "" {
		cloneInd = greenStyle.Render("✓")
	}

	prStr := ""
	if item.PRCount == 1 {
		prStr = "1 PR"
	} else if item.PRCount > 1 {
		prStr = fmt.Sprintf("%d PRs", item.PRCount)
	}

	var line string
	if selected {
		line = "  " + selectedStyle.Render(prefix+nameCol) + " " + cloneInd
	} else {
		line = "  " + normalStyle.Render(prefix+nameCol) + " " + cloneInd
	}
	if prStr != "" {
		line += "  " + orangeStyle.Render(prStr)
	}
	line += "  " + dimStyle.Render(item.Entry.Visibility)
	if item.Entry.UpdatedAt != "" {
		line += "  " + dimStyle.Render(item.Entry.UpdatedAt)
	}
	return line + "\n"
}

func (m model) renderRemoteItem(item DisplayItem, prefix string, selected bool) string {
	name := item.Entry.Name
	if len(name) > 18 {
		name = name[:17] + "…"
	}
	nameCol := fmt.Sprintf("%-18s", name)
	desc := item.Entry.Description

	// Connectivity dot
	dot := dimStyle.Render("○") // unknown
	if reachable, probed := m.deviceStatus[item.Entry.Name]; probed {
		if reachable {
			dot = greenStyle.Render("●")
		} else {
			dot = errStyle.Render("●")
		}
	}

	if selected {
		return "  " + selectedStyle.Render(prefix) + dot + " " + selectedStyle.Render(nameCol) + " " + dimStyle.Render(desc) + "\n"
	}
	return "  " + cyanStyle.Render(prefix) + dot + " " + cyanStyle.Render(nameCol) + " " + dimStyle.Render(desc) + "\n"
}

func (m model) renderDirItem(item DisplayItem, prefix string, selected bool) string {
	name := item.Entry.Name
	if len(name) > 20 {
		name = name[:19] + "…"
	}
	nameCol := fmt.Sprintf("%-20s", name)
	if selected {
		return "  " + selectedStyle.Render(prefix+nameCol) + "  " + dimStyle.Render("—") + "\n"
	}
	return "  " + dimStyle.Render(prefix+nameCol) + "  " + dimStyle.Render("—") + "\n"
}

// ---------------------------------------------------------------------------
// View — Level 1 (repo detail)
// ---------------------------------------------------------------------------

func (m model) viewRepoDetail() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	if m.activeItem == nil {
		return "  (no item selected)\n"
	}

	// Breadcrumb
	crumb := breadStyle.Render("  Atlas") + dimStyle.Render(" > ") + titleStyle.Render(m.activeItem.Entry.Name)
	cloneInfo := ""
	if localPath := m.itemLocalPath(); localPath != "" {
		short := localPath
		home, _ := os.UserHomeDir()
		if strings.HasPrefix(short, home) {
			short = "~" + short[len(home):]
		}
		cloneInfo = greenStyle.Render("✓ ") + dimStyle.Render(short)
	} else {
		cloneInfo = dimStyle.Render("✗ not cloned")
	}
	pad := w - lipgloss.Width(crumb) - lipgloss.Width(cloneInfo)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(crumb + strings.Repeat(" ", pad) + cloneInfo + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", maxInt(0, w-4))) + "\n")

	// Loading
	if m.detailLoading && len(m.prs) == 0 && len(m.branches) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Loading details...\n")
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("  Esc=back  q=quit"))
		b.WriteString("\n")
		return b.String()
	}

	// Error
	if m.detailErr != nil {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.detailErr.Error()) + "\n")
	}

	// PRs section
	b.WriteString("\n")
	prHeader := "Open PRs"
	if len(m.prs) > 0 {
		prHeader = fmt.Sprintf("Open PRs (%d)", len(m.prs))
	}
	if m.detailSection == SectionPRs {
		b.WriteString("  " + sectionStyle.Render(prHeader) + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render(prHeader) + "\n")
	}

	if !m.itemIsGithub() {
		b.WriteString("  " + dimStyle.Render("  local only — no PRs") + "\n")
	} else if len(m.prs) == 0 {
		b.WriteString("  " + dimStyle.Render("  no open PRs") + "\n")
	}
	for i, pr := range m.prs {
		selected := m.detailSection == SectionPRs && i == m.detailCursor
		prefix := "    "
		if selected {
			prefix = "  ▸ "
		}

		num := fmt.Sprintf("#%-4d", pr.Number)
		title := pr.Title
		maxTitle := w - 30
		if maxTitle < 10 {
			maxTitle = 10
		}
		if len(title) > maxTitle {
			title = title[:maxTitle-1] + "…"
		}

		author := ""
		if pr.Author.Login != "" {
			author = "@" + pr.Author.Login
		}

		review := ""
		switch pr.ReviewDecision {
		case "APPROVED":
			review = greenStyle.Render("APPROVED")
		case "CHANGES_REQUESTED":
			review = errStyle.Render("CHANGES")
		case "REVIEW_REQUIRED":
			review = orangeStyle.Render("PENDING")
		}
		if pr.IsDraft {
			review = dimStyle.Render("DRAFT")
		}

		ago := ""
		if t, err := time.Parse(time.RFC3339, pr.CreatedAt); err == nil {
			ago = timeAgo(t)
		}

		meta := dimStyle.Render(fmt.Sprintf("  %s  %s", author, ago))
		if review != "" {
			meta += "  " + review
		}

		if selected {
			b.WriteString(prefix + selectedStyle.Render(num+" "+title) + meta + "\n")
		} else {
			b.WriteString(prefix + normalStyle.Render(num) + " " + normalStyle.Render(title) + meta + "\n")
		}
	}

	// Branches section
	b.WriteString("\n")
	branchHeader := "Branches"
	if len(m.branches) > 0 {
		branchHeader = fmt.Sprintf("Branches (%d)", len(m.branches))
	}
	if m.detailSection == SectionBranches {
		b.WriteString("  " + sectionStyle.Render(branchHeader) + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render(branchHeader) + "\n")
	}

	if len(m.branches) == 0 {
		b.WriteString("  " + dimStyle.Render("  no branches loaded") + "\n")
	}
	for i, branch := range m.branches {
		selected := m.detailSection == SectionBranches && i == m.detailCursor
		prefix := "    "
		if selected {
			prefix = "  ▸ "
		}
		name := branch.Name
		if len(name) > 30 {
			name = name[:29] + "…"
		}
		if selected {
			b.WriteString(prefix + selectedStyle.Render(name) + "\n")
		} else {
			b.WriteString(prefix + normalStyle.Render(name) + "\n")
		}
	}

	// Actions footer
	b.WriteString("\n")
	actions := "  [e] shell"
	if m.itemLocalPath() != "" {
		actions += "   [a] agent   [s] scan"
	} else {
		actions += "   [a] clone+agent"
	}
	actions += "   [o] browser"
	b.WriteString(footerStyle.Render(actions) + "\n")
	b.WriteString(footerStyle.Render("  ↑↓/jk=nav  Tab=section  ⏎=select  Esc=back") + "\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// View — Level 2 (PR detail)
// ---------------------------------------------------------------------------

func (m model) viewPRDetail() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	if m.activeItem == nil || m.activePR == nil {
		return "  (no PR selected)\n"
	}

	// Breadcrumb
	crumb := breadStyle.Render("  Atlas") + dimStyle.Render(" > ") +
		breadStyle.Render(m.activeItem.Entry.Name) + dimStyle.Render(" > ") +
		titleStyle.Render(fmt.Sprintf("PR #%d", m.activePR.Number))
	b.WriteString(crumb + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", maxInt(0, w-4))) + "\n")

	// Loading
	if m.prLoading {
		b.WriteString("\n  " + m.spinner.View() + " Loading PR...\n")
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("  Esc=back  q=quit"))
		b.WriteString("\n")
		return b.String()
	}

	// Error
	if m.prErr != nil {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.prErr.Error()) + "\n")
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("  Esc=back  q=quit"))
		b.WriteString("\n")
		return b.String()
	}

	if m.prDetail == nil {
		b.WriteString("\n  " + dimStyle.Render("no data") + "\n")
		b.WriteString(footerStyle.Render("  Esc=back"))
		b.WriteString("\n")
		return b.String()
	}

	d := m.prDetail

	// Title
	b.WriteString("\n")
	title := d.Title
	maxTitle := w - 4
	if len(title) > maxTitle {
		title = title[:maxTitle-1] + "…"
	}
	b.WriteString("  " + selectedStyle.Render(title) + "\n")

	// Author + age + review
	author := ""
	if d.Author.Login != "" {
		author = "by @" + d.Author.Login
	}
	ago := ""
	if t, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
		ago = timeAgo(t) + " ago"
	}

	review := ""
	switch d.ReviewDecision {
	case "APPROVED":
		review = greenStyle.Render("APPROVED")
	case "CHANGES_REQUESTED":
		review = errStyle.Render("CHANGES REQUESTED")
	case "REVIEW_REQUIRED":
		review = orangeStyle.Render("REVIEW PENDING")
	}

	metaParts := []string{}
	if author != "" {
		metaParts = append(metaParts, author)
	}
	if ago != "" {
		metaParts = append(metaParts, ago)
	}
	metaLine := strings.Join(metaParts, "  ·  ")
	b.WriteString("  " + dimStyle.Render(metaLine))
	if review != "" {
		b.WriteString("  ·  " + review)
	}
	b.WriteString("\n")

	// Branch
	b.WriteString("  " + dimStyle.Render("branch: ") + cyanStyle.Render(d.HeadRefName) + "\n")

	// Diff stats
	b.WriteString("\n")
	diffLine := fmt.Sprintf("  %s  %s  ·  %d files changed",
		greenStyle.Render(fmt.Sprintf("+%d", d.Additions)),
		errStyle.Render(fmt.Sprintf("-%d", d.Deletions)),
		d.ChangedFiles)
	b.WriteString(diffLine + "\n")

	// Checks summary
	if len(d.StatusCheckRollup) > 0 {
		passed := 0
		failed := 0
		pending := 0
		for _, c := range d.StatusCheckRollup {
			switch {
			case c.Conclusion == "SUCCESS" || c.State == "SUCCESS":
				passed++
			case c.Conclusion == "FAILURE" || c.State == "FAILURE":
				failed++
			default:
				pending++
			}
		}
		total := len(d.StatusCheckRollup)
		checkStr := fmt.Sprintf("  Checks: %d/%d passed", passed, total)
		if failed > 0 {
			checkStr += fmt.Sprintf("  %s", errStyle.Render(fmt.Sprintf("%d failed", failed)))
		}
		if pending > 0 {
			checkStr += fmt.Sprintf("  %s", orangeStyle.Render(fmt.Sprintf("%d pending", pending)))
		}
		if passed == total {
			checkStr += " " + greenStyle.Render("✓")
		}
		b.WriteString(dimStyle.Render(checkStr) + "\n")
	}

	// Body excerpt
	if d.Body != "" {
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render("Description:") + "\n")
		lines := strings.Split(d.Body, "\n")
		maxLines := 8
		if m.height > 0 {
			maxLines = maxInt(4, m.height-18)
		}
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		for _, line := range lines {
			if len(line) > w-4 {
				line = line[:w-5] + "…"
			}
			b.WriteString("  " + dimStyle.Render(line) + "\n")
		}
		if len(strings.Split(d.Body, "\n")) > maxLines {
			b.WriteString("  " + dimStyle.Render("...") + "\n")
		}
	}

	// Actions
	b.WriteString("\n")
	actions := "  [e] checkout"
	if m.itemLocalPath() != "" {
		actions += "   [a] agent on branch"
	}
	actions += "   [o] browser   Esc=back"
	b.WriteString(footerStyle.Render(actions) + "\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// View — Remote detail
// ---------------------------------------------------------------------------

func (m model) viewRemoteDetail() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	if m.activeItem == nil {
		return "  (no host selected)\n"
	}

	e := m.activeItem.Entry

	// Breadcrumb
	crumb := breadStyle.Render("  Atlas") + dimStyle.Render(" > ") + titleStyle.Render(e.Name)
	b.WriteString(crumb + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", maxInt(0, w-4))) + "\n")

	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render("Host:  ") + normalStyle.Render(e.Host) + "\n")
	b.WriteString("  " + dimStyle.Render("User:  ") + normalStyle.Render(e.User) + "\n")
	if e.OS != "" {
		b.WriteString("  " + dimStyle.Render("OS:    ") + normalStyle.Render(e.OS) + "\n")
	}
	if e.Description != "" {
		b.WriteString("\n")
		b.WriteString("  " + normalStyle.Render(e.Description) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  [e] SSH connect   [a] remote agent   Esc=back   q=quit") + "\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/24/7))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
	}
}

func maxInt(a, b int) int {
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
