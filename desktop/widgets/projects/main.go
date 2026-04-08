package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

	_ = cyanStyle
)

// ---------------------------------------------------------------------------
// View levels
// ---------------------------------------------------------------------------

type ViewLevel int

const (
	LevelRepoList  ViewLevel = iota
	LevelRepoDetail
	LevelPRDetail
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

type Repo struct {
	Name             string `json:"name"`
	DefaultBranchRef struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
	PushedAt  string `json:"pushedAt"`
	URL       string `json:"url"`
	PRCount   int
	LocalPath string
}

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
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	HeadRefName  string `json:"headRefName"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	ChangedFiles int    `json:"changedFiles"`
	ReviewDecision string `json:"reviewDecision"`
	URL          string `json:"url"`
	Author       struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt string `json:"createdAt"`
	StatusCheckRollup []struct {
		Context    string `json:"context"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

// ---------------------------------------------------------------------------
// Tea messages
// ---------------------------------------------------------------------------

type tickMsg time.Time

type repoListMsg struct {
	repos []Repo
	err   error
}

type cloneMapMsg struct {
	cloneMap map[string]string
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

	cloneMap map[string]string

	// Navigation
	viewLevel ViewLevel

	// Level 0 — Repo list
	repos       []Repo
	repoCursor  int
	repoLoading bool
	repoErr     error

	// Level 1 — Repo detail
	activeRepo    *Repo
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
		spinner:  s,
		cloneMap: make(map[string]string),
		repoLoading: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchReposCmd(), detectClonesCmd(), scheduleTickCmd())
}

// ---------------------------------------------------------------------------
// Commands — Level 0
// ---------------------------------------------------------------------------

func getOwner() (string, error) {
	out, err := exec.Command("gh", "api", "user", "--jq", ".login").Output()
	if err != nil {
		return "", fmt.Errorf("gh api user failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func fetchReposCmd() tea.Cmd {
	return func() tea.Msg {
		owner, err := getOwner()
		if err != nil {
			return repoListMsg{err: err}
		}

		out, err := exec.Command("gh", "repo", "list",
			"--json", "name,defaultBranchRef,pushedAt,url",
			"--limit", "15",
			"--source",
		).CombinedOutput()
		if err != nil {
			return repoListMsg{err: fmt.Errorf("gh: %s", strings.TrimSpace(string(out)))}
		}

		var repos []Repo
		if err := json.Unmarshal(out, &repos); err != nil {
			return repoListMsg{err: fmt.Errorf("JSON parse: %w", err)}
		}

		sort.Slice(repos, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, repos[i].PushedAt)
			tj, _ := time.Parse(time.RFC3339, repos[j].PushedAt)
			return ti.After(tj)
		})

		// Fetch PR counts in parallel
		var wg sync.WaitGroup
		for i := range repos {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				fullName := owner + "/" + repos[idx].Name
				prOut, prErr := exec.Command("gh", "pr", "list",
					"--repo", fullName,
					"--state", "open",
					"--json", "number",
					"--limit", "100",
				).Output()
				if prErr != nil {
					return
				}
				var prs []struct{ Number int }
				if json.Unmarshal(prOut, &prs) == nil {
					repos[idx].PRCount = len(prs)
				}
			}(i)
		}
		wg.Wait()

		return repoListMsg{repos: repos}
	}
}

func detectClonesCmd() tea.Cmd {
	return func() tea.Msg {
		cloneMap := make(map[string]string)
		home, _ := os.UserHomeDir()

		searchDirs := []string{home}
		// Also check common code directories
		for _, sub := range []string{"code", "projects", "src", "dev", "repos", "Desktop", "Documents"} {
			p := filepath.Join(home, sub)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				searchDirs = append(searchDirs, p)
			}
		}

		for _, base := range searchDirs {
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				dir := filepath.Join(base, e.Name())
				gitDir := filepath.Join(dir, ".git")
				if _, err := os.Stat(gitDir); err != nil {
					continue
				}
				out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
				if err != nil {
					continue
				}
				name := extractRepoName(strings.TrimSpace(string(out)))
				if name != "" {
					// Prefer shorter paths (closer to $HOME)
					if existing, ok := cloneMap[name]; !ok || len(dir) < len(existing) {
						cloneMap[name] = dir
					}
				}
			}
		}

		return cloneMapMsg{cloneMap: cloneMap}
	}
}

func extractRepoName(remoteURL string) string {
	// Handle SSH: git@github.com:user/repo.git
	if idx := strings.LastIndex(remoteURL, ":"); idx > 0 && strings.Contains(remoteURL, "@") {
		path := remoteURL[idx+1:]
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[len(parts)-1]
		}
	}
	// Handle HTTPS: https://github.com/user/repo.git
	if strings.Contains(remoteURL, "://") {
		path := remoteURL
		if idx := strings.Index(path, "://"); idx >= 0 {
			path = path[idx+3:]
		}
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) >= 3 {
			return parts[len(parts)-1]
		}
	}
	return ""
}

func scheduleTickCmd() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Commands — Level 1
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
				// Fallback: try simpler approach
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

// ---------------------------------------------------------------------------
// Commands — Level 2
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

func shellInRepoCmd(localPath string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("tmux", "new-window", "-n", filepath.Base(localPath), "-c", localPath).Start()
		return nil
	}
}

func agentInRepoCmd(localPath string) tea.Cmd {
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
		m.repoLoading = true
		m.repoErr = nil
		return m, tea.Batch(m.spinner.Tick, fetchReposCmd(), scheduleTickCmd())

	case repoListMsg:
		m.repoLoading = false
		if msg.err != nil {
			m.repoErr = msg.err
		} else {
			// Preserve owner from first successful fetch
			if m.owner == "" {
				if o, err := getOwner(); err == nil {
					m.owner = o
				}
			}
			m.repos = msg.repos
			m.repoErr = nil
			// Merge clone paths
			for i := range m.repos {
				if p, ok := m.cloneMap[m.repos[i].Name]; ok {
					m.repos[i].LocalPath = p
				}
			}
			if m.repoCursor >= len(m.repos) {
				m.repoCursor = maxInt(0, len(m.repos)-1)
			}
		}
		return m, nil

	case cloneMapMsg:
		m.cloneMap = msg.cloneMap
		// Merge into existing repos
		for i := range m.repos {
			if p, ok := m.cloneMap[m.repos[i].Name]; ok {
				m.repos[i].LocalPath = p
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
		case LevelRepoList:
			return m.updateRepoList(msg)
		case LevelRepoDetail:
			return m.updateRepoDetail(msg)
		case LevelPRDetail:
			return m.updatePRDetail(msg)
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Level 0
// ---------------------------------------------------------------------------

func (m model) updateRepoList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}

	case "k", "up":
		if m.repoCursor > 0 {
			m.repoCursor--
		}

	case "enter":
		if len(m.repos) > 0 && m.owner != "" {
			repo := m.repos[m.repoCursor]
			m.activeRepo = &repo
			m.viewLevel = LevelRepoDetail
			m.detailLoading = true
			m.detailCursor = 0
			m.detailSection = SectionPRs
			m.prs = nil
			m.branches = nil
			m.detailErr = nil
			return m, tea.Batch(m.spinner.Tick, fetchRepoDetailCmd(m.owner, repo.Name))
		}

	case "o":
		if len(m.repos) > 0 {
			return m, openBrowserCmd(m.repos[m.repoCursor].URL)
		}

	case "e":
		if len(m.repos) > 0 {
			repo := m.repos[m.repoCursor]
			if repo.LocalPath != "" {
				return m, shellInRepoCmd(repo.LocalPath)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, repo.Name)
			}
		}

	case "a":
		if len(m.repos) > 0 {
			repo := m.repos[m.repoCursor]
			if repo.LocalPath != "" {
				return m, agentInRepoCmd(repo.LocalPath)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, repo.Name)
			}
		}

	case "r":
		m.repoLoading = true
		m.repoErr = nil
		return m, tea.Batch(m.spinner.Tick, fetchReposCmd(), detectClonesCmd())
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Update — Level 1
// ---------------------------------------------------------------------------

func (m model) updateRepoDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "backspace":
		m.viewLevel = LevelRepoList
		m.activeRepo = nil
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
			return m, tea.Batch(m.spinner.Tick, fetchPRDetailCmd(m.owner, m.activeRepo.Name, pr.Number))
		}
		if m.detailSection == SectionBranches && m.detailCursor < len(m.branches) {
			branch := m.branches[m.detailCursor]
			if m.activeRepo.LocalPath != "" {
				return m, func() tea.Msg {
					name := filepath.Base(m.activeRepo.LocalPath) + "/" + branch.Name
					if len(name) > 20 {
						name = branch.Name
					}
					_ = exec.Command("tmux", "new-window", "-n", name, "-c", m.activeRepo.LocalPath,
						"bash", "-ic", fmt.Sprintf("git checkout %s 2>/dev/null || git checkout -b %s && exec bash", branch.Name, branch.Name)).Start()
					return nil
				}
			}
		}

	case "e":
		if m.activeRepo != nil {
			if m.activeRepo.LocalPath != "" {
				return m, shellInRepoCmd(m.activeRepo.LocalPath)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, m.activeRepo.Name)
			}
		}

	case "a":
		if m.activeRepo != nil {
			if m.activeRepo.LocalPath != "" {
				return m, agentInRepoCmd(m.activeRepo.LocalPath)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, m.activeRepo.Name)
			}
		}

	case "s":
		if m.activeRepo != nil && m.activeRepo.LocalPath != "" {
			return m, scanRepoCmd(m.activeRepo.LocalPath)
		}

	case "o":
		if m.activeRepo != nil {
			return m, openBrowserCmd(m.activeRepo.URL)
		}

	case "r":
		if m.activeRepo != nil && m.owner != "" {
			m.detailLoading = true
			m.detailErr = nil
			return m, tea.Batch(m.spinner.Tick, fetchRepoDetailCmd(m.owner, m.activeRepo.Name))
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
// Update — Level 2
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
		if m.activeRepo != nil && m.activePR != nil {
			if m.activeRepo.LocalPath != "" {
				return m, checkoutPRCmd(m.activeRepo.LocalPath, m.activePR.Number)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, m.activeRepo.Name)
			}
		}

	case "a":
		if m.activeRepo != nil && m.activePR != nil {
			if m.activeRepo.LocalPath != "" {
				return m, agentOnPRCmd(m.activeRepo.LocalPath, m.activePR.Number)
			} else if m.owner != "" {
				return m, cloneAndOpenCmd(m.owner, m.activeRepo.Name)
			}
		}

	case "o":
		if m.prDetail != nil && m.prDetail.URL != "" {
			return m, openBrowserCmd(m.prDetail.URL)
		} else if m.activeRepo != nil {
			return m, openBrowserCmd(m.activeRepo.URL)
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	switch m.viewLevel {
	case LevelRepoList:
		return m.viewRepoList()
	case LevelRepoDetail:
		return m.viewRepoDetail()
	case LevelPRDetail:
		return m.viewPRDetail()
	}
	return ""
}

// ---------------------------------------------------------------------------
// View — Level 0
// ---------------------------------------------------------------------------

func (m model) viewRepoList() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	// Title row
	title := titleStyle.Render("  Projects")
	countStr := ""
	if len(m.repos) > 0 {
		countStr = dimStyle.Render(fmt.Sprintf("%d repos", len(m.repos)))
	}
	titlePad := w - lipgloss.Width(title) - lipgloss.Width(countStr)
	if titlePad < 1 {
		titlePad = 1
	}
	b.WriteString(title + strings.Repeat(" ", titlePad) + countStr + "\n")
	b.WriteString(dimStyle.Render("  "+strings.Repeat("─", maxInt(0, w-4))) + "\n")

	// Loading
	if m.repoLoading && len(m.repos) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Loading projects...\n")
		return b.String()
	}

	// Error
	if m.repoErr != nil && len(m.repos) == 0 {
		b.WriteString("\n  " + errStyle.Render("Error: "+m.repoErr.Error()) + "\n")
		b.WriteString("\n  " + dimStyle.Render("Press r to retry, q to quit") + "\n")
		return b.String()
	}

	// Repo list
	for i, repo := range m.repos {
		selected := i == m.repoCursor
		prefix := "  "
		if selected {
			prefix = "▸ "
		}

		name := repo.Name
		if len(name) > 20 {
			name = name[:19] + "…"
		}

		// PR count
		prStr := ""
		if repo.PRCount == 1 {
			prStr = "1 PR"
		} else if repo.PRCount > 1 {
			prStr = fmt.Sprintf("%d PRs", repo.PRCount)
		}

		// Clone indicator
		cloneInd := dimStyle.Render("✗")
		if repo.LocalPath != "" {
			cloneInd = greenStyle.Render("✓")
		}

		// Time ago
		ago := ""
		if t, err := time.Parse(time.RFC3339, repo.PushedAt); err == nil {
			ago = timeAgo(t)
		}

		nameCol := fmt.Sprintf("%-20s", name)
		prCol := fmt.Sprintf("%-6s", prStr)
		usedWidth := 2 + 20 + 1 + 6 + 1 + 2 + 1 // prefix+name+gap+pr+gap+clone+gap
		agoWidth := w - usedWidth - 1
		if agoWidth < 0 {
			agoWidth = 0
		}
		agoCol := fmt.Sprintf("%*s", agoWidth, ago)

		if selected {
			b.WriteString(selectedStyle.Render(prefix+nameCol) + " " +
				orangeStyle.Render(prCol) + " " + cloneInd + " " +
				dimStyle.Render(agoCol) + "\n")
		} else {
			b.WriteString(normalStyle.Render(prefix+nameCol) + " " +
				orangeStyle.Render(prCol) + " " + cloneInd + " " +
				dimStyle.Render(agoCol) + "\n")
		}
	}

	// Refresh indicator
	if m.repoLoading && len(m.repos) > 0 {
		b.WriteString("\n  " + m.spinner.View() + " refreshing...")
	}
	if m.repoErr != nil && len(m.repos) > 0 {
		b.WriteString("\n  " + errStyle.Render("refresh failed: "+m.repoErr.Error()))
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  ↑↓/jk=nav  ⏎=detail  o=web  e=shell  a=agent  r=refresh  q=quit"))
	b.WriteString("\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// View — Level 1
// ---------------------------------------------------------------------------

func (m model) viewRepoDetail() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	if m.activeRepo == nil {
		return "  (no repo selected)\n"
	}

	// Breadcrumb
	crumb := breadStyle.Render("  Projects") + dimStyle.Render(" > ") + titleStyle.Render(m.activeRepo.Name)
	cloneInfo := ""
	if m.activeRepo.LocalPath != "" {
		short := m.activeRepo.LocalPath
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

	if len(m.prs) == 0 {
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
		maxTitle := w - 30 // leave room for metadata
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
	if m.activeRepo.LocalPath != "" {
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
// View — Level 2
// ---------------------------------------------------------------------------

func (m model) viewPRDetail() string {
	var b strings.Builder
	w := m.width
	if w <= 0 {
		w = 60
	}

	if m.activeRepo == nil || m.activePR == nil {
		return "  (no PR selected)\n"
	}

	// Breadcrumb
	crumb := breadStyle.Render("  Projects") + dimStyle.Render(" > ") +
		breadStyle.Render(m.activeRepo.Name) + dimStyle.Render(" > ") +
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
		body := d.Body
		// Show first ~8 lines
		lines := strings.Split(body, "\n")
		maxLines := 8
		if m.height > 0 {
			maxLines = maxInt(4, (m.height-18))
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
	if m.activeRepo.LocalPath != "" {
		actions += "   [a] agent on branch"
	}
	actions += "   [o] browser   Esc=back"
	b.WriteString(footerStyle.Render(actions) + "\n")

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
