package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
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
	selectedStyle = lipgloss.NewStyle().Foreground(bright).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(white)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	footerStyle   = lipgloss.NewStyle().Foreground(dim)
	scoreStyle    = lipgloss.NewStyle().Foreground(orange).Bold(true)
	greenStyle    = lipgloss.NewStyle().Foreground(green)

	_ = cyan  // available for future use
	_ = red   // used in errStyle below
	_ = green // used in greenStyle above

	errStyle = lipgloss.NewStyle().Foreground(red)
)

// ---------------------------------------------------------------------------
// HN data types
// ---------------------------------------------------------------------------

type Story struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Descendants int    `json:"descendants"`
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type tickMsg time.Time

type storiesMsg struct {
	stories []Story
	err     error
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

var httpClient = &http.Client{Timeout: 10 * time.Second}

func fetchCmd() tea.Cmd {
	return func() tea.Msg {
		stories, err := fetchTopStories(25)
		return storiesMsg{stories: stories, err: err}
	}
}

func fetchTopStories(n int) ([]Story, error) {
	resp, err := httpClient.Get("https://hacker-news.firebaseio.com/v0/topstories.json")
	if err != nil {
		return nil, fmt.Errorf("fetching top stories: %w", err)
	}
	defer resp.Body.Close()

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decoding story IDs: %w", err)
	}

	if len(ids) > n {
		ids = ids[:n]
	}

	type result struct {
		index int
		story Story
		err   error
	}

	ch := make(chan result, len(ids))
	for i, id := range ids {
		go func(idx, storyID int) {
			s, err := fetchStory(storyID)
			ch <- result{index: idx, story: s, err: err}
		}(i, id)
	}

	stories := make([]Story, len(ids))
	var firstErr error
	for range ids {
		r := <-ch
		if r.err != nil && firstErr == nil {
			firstErr = r.err
			continue
		}
		stories[r.index] = r.story
	}

	// Filter out zero-value stories (failed fetches) while preserving order.
	filtered := make([]Story, 0, len(stories))
	for _, s := range stories {
		if s.ID != 0 {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return filtered, nil
}

func fetchStory(id int) (Story, error) {
	url := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id)
	resp, err := httpClient.Get(url)
	if err != nil {
		return Story{}, err
	}
	defer resp.Body.Close()

	var s Story
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return Story{}, err
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

func extractDomain(rawURL string) string {
	if rawURL == "" {
		return "news.ycombinator.com"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "news.ycombinator.com"
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	if host == "" {
		return "news.ycombinator.com"
	}
	return host
}

func timeAgo(unix int64) string {
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	}
}

func scheduleTick() tea.Cmd {
	return tea.Tick(10*time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	stories []Story
	cursor  int
	offset  int
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
	return model{
		spinner: s,
		loading: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchCmd(), scheduleTick())
}

// visibleCount returns how many stories fit on screen.
// Each story occupies 2 lines; subtract 6 for header + footer chrome.
func (m model) visibleCount() int {
	if m.height <= 6 {
		return 1
	}
	v := (m.height - 6) / 2
	if v < 1 {
		return 1
	}
	return v
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j", "down":
			if m.cursor < len(m.stories)-1 {
				m.cursor++
				vis := m.visibleCount()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}

		case "enter":
			if m.cursor < len(m.stories) {
				s := m.stories[m.cursor]
				u := s.URL
				if u == "" {
					u = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", s.ID)
				}
				_ = exec.Command("open", u).Start()
			}

		case "c":
			if m.cursor < len(m.stories) {
				s := m.stories[m.cursor]
				u := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", s.ID)
				_ = exec.Command("open", u).Start()
			}

		case "r":
			m.loading = true
			m.err = nil
			return m, tea.Batch(m.spinner.Tick, fetchCmd())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.loading = true
		m.err = nil
		return m, tea.Batch(m.spinner.Tick, fetchCmd(), scheduleTick())

	case storiesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			// Keep previous stories on refresh failure.
		} else {
			m.stories = msg.stories
			m.err = nil
			// Clamp cursor/offset to new data length.
			if m.cursor >= len(m.stories) {
				m.cursor = max(0, len(m.stories)-1)
			}
			if m.offset > m.cursor {
				m.offset = m.cursor
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// ── Header ──────────────────────────────────────────────────────────
	title := titleStyle.Render("  News")
	count := ""
	if len(m.stories) > 0 {
		count = subtitleStyle.Render(fmt.Sprintf("%d stories", len(m.stories)))
	}
	headerWidth := m.width
	if headerWidth <= 0 {
		headerWidth = 60
	}
	padding := headerWidth - lipgloss.Width(title) - lipgloss.Width(count)
	if padding < 1 {
		padding = 1
	}
	b.WriteString(title + strings.Repeat(" ", padding) + count + "\n")
	b.WriteString(dimStyle.Render("  " + strings.Repeat("─", max(0, headerWidth-4))) + "\n")

	// ── Loading / Error ─────────────────────────────────────────────────
	if m.loading && len(m.stories) == 0 {
		b.WriteString("\n  " + m.spinner.View() + " Loading stories...\n")
		b.WriteString("\n")
		b.WriteString(footer())
		return b.String()
	}

	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("\n  Error: %s\n", m.err)) + "\n")
	}

	if len(m.stories) == 0 {
		b.WriteString("\n  No stories loaded.\n")
		b.WriteString("\n")
		b.WriteString(footer())
		return b.String()
	}

	// ── Story list ──────────────────────────────────────────────────────
	vis := m.visibleCount()
	end := m.offset + vis
	if end > len(m.stories) {
		end = len(m.stories)
	}

	// Scroll-up indicator.
	if m.offset > 0 {
		b.WriteString(dimStyle.Render("  ▲ more") + "\n")
	}

	for i := m.offset; i < end; i++ {
		s := m.stories[i]
		selected := i == m.cursor

		// Line 1: [▸] score  title
		prefix := "  "
		if selected {
			prefix = "▸ "
		}

		scoreStr := scoreStyle.Render(fmt.Sprintf("%4d", s.Score))

		var titleStr string
		if selected {
			titleStr = selectedStyle.Render("  " + s.Title)
		} else {
			titleStr = normalStyle.Render("  " + s.Title)
		}

		// Truncate title if needed to fit width.
		maxTitleWidth := headerWidth - 8 // prefix(2) + score(4) + gap(2)
		rendered := prefix + scoreStr + titleStr
		if lipgloss.Width(rendered) > headerWidth && maxTitleWidth > 3 {
			truncated := truncateStr(s.Title, maxTitleWidth)
			if selected {
				titleStr = selectedStyle.Render("  " + truncated)
			} else {
				titleStr = normalStyle.Render("  " + truncated)
			}
		}

		b.WriteString(prefix + scoreStr + titleStr + "\n")

		// Line 2: domain + comments + age
		domain := extractDomain(s.URL)
		meta := fmt.Sprintf("%s \u00b7 %d comments \u00b7 %s ago",
			domain, s.Descendants, timeAgo(s.Time))
		b.WriteString(dimStyle.Render("       "+meta) + "\n")
	}

	// Scroll-down indicator.
	if end < len(m.stories) {
		b.WriteString(dimStyle.Render("  ▼ more") + "\n")
	}

	// ── Footer ──────────────────────────────────────────────────────────
	// Pad to fill remaining height so footer sits at bottom.
	usedLines := 2 // header
	if m.offset > 0 {
		usedLines++
	}
	usedLines += (end - m.offset) * 2
	if end < len(m.stories) {
		usedLines++
	}
	if m.err != nil {
		usedLines += 2
	}
	usedLines++ // footer itself
	remaining := m.height - usedLines - 1
	for i := 0; i < remaining; i++ {
		b.WriteString("\n")
	}

	b.WriteString(footer())

	return b.String()
}

func footer() string {
	return footerStyle.Render("  \u2191\u2193/jk=nav  \u23ce=open  c=comments  r=refresh  q=quit")
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

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
		fmt.Printf("Error: %v\n", err)
	}
}
