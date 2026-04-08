package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed public
var publicFS embed.FS

// ---------------------------------------------------------------------------
// Data types matching omacmux state files
// ---------------------------------------------------------------------------

type SwarmState struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Topology   string      `json:"topology"`
	Hub        string      `json:"hub,omitempty"`
	Status     string      `json:"status"`
	Conductor  string      `json:"conductor,omitempty"`
	WorkingDir string      `json:"working_dir,omitempty"`
	WindowID   string      `json:"window_id,omitempty"`
	Session    string      `json:"session,omitempty"`
	Created    string      `json:"created,omitempty"`
	Agents     interface{} `json:"agents"`
	Mailbox    []MailMsg   `json:"mailbox,omitempty"`
}

type AgentState struct {
	ID         string  `json:"id"`
	PaneID     string  `json:"pane_id"`
	Role       string  `json:"role"`
	Command    string  `json:"command,omitempty"`
	Device     string  `json:"device,omitempty"`
	RemotePane *string `json:"remote_pane,omitempty"`
	Status     string  `json:"status"`
	Started    string  `json:"started,omitempty"`
	Branch     string  `json:"branch,omitempty"`
	NATO       string  `json:"_nato,omitempty"`
	SwarmID    string  `json:"_swarm_id,omitempty"`
	SwarmName  string  `json:"_swarm_name,omitempty"`
}

type MailMsg struct {
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type SSEEvent struct {
	Type      string      `json:"type"`
	SwarmID   string      `json:"swarm_id,omitempty"`
	AgentID   string      `json:"agent_id,omitempty"`
	AgentName string      `json:"agent_name,omitempty"`
	Name      string      `json:"name,omitempty"`
	Status    string      `json:"status,omitempty"`
	Role      string      `json:"role,omitempty"`
	Branch    string      `json:"branch,omitempty"`
	Command   string      `json:"command,omitempty"`
	Message   string      `json:"message,omitempty"`
	Success   *bool       `json:"success,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp string      `json:"timestamp"`
}

type CommandRequest struct {
	Cmd string `json:"cmd"`
}

type TellRequest struct {
	Msg string `json:"msg"`
}

type CommandResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// NATO name mapping
// ---------------------------------------------------------------------------

var natoNames = map[string]string{
	"alpha":   "agent-1",
	"bravo":   "agent-2",
	"charlie": "agent-3",
	"delta":   "agent-4",
	"echo":    "agent-5",
	"foxtrot": "agent-6",
	"golf":    "agent-7",
	"hotel":   "agent-8",
}

var natoReverse = map[string]string{
	"agent-1": "alpha",
	"agent-2": "bravo",
	"agent-3": "charlie",
	"agent-4": "delta",
	"agent-5": "echo",
	"agent-6": "foxtrot",
	"agent-7": "golf",
	"agent-8": "hotel",
}

// ---------------------------------------------------------------------------
// Command allowlist
// ---------------------------------------------------------------------------

var allowedCommands = map[string]bool{
	"check":  true,
	"who":    true,
	"vibe":   true,
	"tell":   true,
	"scan":   true,
	"swarm":  true,
	"review": true,
	"recap":  true,
	"focus":  true,
	"ship":   true,
}

// ---------------------------------------------------------------------------
// SVG Icon
// ---------------------------------------------------------------------------

const iconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect width="512" height="512" rx="96" fill="#0d1117"/>
  <circle cx="176" cy="176" r="48" fill="#3fb950"/>
  <circle cx="336" cy="176" r="48" fill="#58a6ff"/>
  <circle cx="176" cy="336" r="48" fill="#d29922"/>
  <circle cx="336" cy="336" r="48" fill="#bc8cff"/>
  <line x1="176" y1="176" x2="336" y2="176" stroke="#30363d" stroke-width="4"/>
  <line x1="176" y1="176" x2="176" y2="336" stroke="#30363d" stroke-width="4"/>
  <line x1="336" y1="176" x2="336" y2="336" stroke="#30363d" stroke-width="4"/>
  <line x1="176" y1="336" x2="336" y2="336" stroke="#30363d" stroke-width="4"/>
  <line x1="176" y1="176" x2="336" y2="336" stroke="#30363d" stroke-width="3" stroke-dasharray="8,4"/>
  <line x1="336" y1="176" x2="176" y2="336" stroke="#30363d" stroke-width="3" stroke-dasharray="8,4"/>
</svg>`

// ---------------------------------------------------------------------------
// SSE Broker with event history
// ---------------------------------------------------------------------------

type SSEBroker struct {
	mu       sync.RWMutex
	clients  map[chan SSEEvent]struct{}
	history  []SSEEvent
	maxHist  int
	replayN  int
}

func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan SSEEvent]struct{}),
		maxHist: 200,
		replayN: 50,
	}
}

func (b *SSEBroker) Subscribe() (chan SSEEvent, []SSEEvent) {
	ch := make(chan SSEEvent, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	// Copy recent history for replay
	start := 0
	if len(b.history) > b.replayN {
		start = len(b.history) - b.replayN
	}
	replay := make([]SSEEvent, len(b.history[start:]))
	copy(replay, b.history[start:])
	b.mu.Unlock()
	return ch, replay
}

func (b *SSEBroker) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *SSEBroker) Publish(event SSEEvent) {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	b.mu.Lock()
	b.history = append(b.history, event)
	if len(b.history) > b.maxHist {
		b.history = b.history[len(b.history)-b.maxHist:]
	}
	b.mu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
			// drop if client is slow
		}
	}
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

type Server struct {
	stateDir string
	broker   *SSEBroker
	mux      *http.ServeMux
	staticFS http.FileSystem
}

func NewServer(stateDir string) *Server {
	subFS, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal("failed to create sub filesystem:", err)
	}

	s := &Server{
		stateDir: stateDir,
		broker:   NewSSEBroker(),
		mux:      http.NewServeMux(),
		staticFS: http.FS(subFS),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/swarms", s.handleSwarms)
	s.mux.HandleFunc("/api/swarm/", s.handleSwarmDetail)
	s.mux.HandleFunc("/api/agent/", s.handleAgentDetail)
	s.mux.HandleFunc("/api/pane/", s.handlePaneCapture)
	s.mux.HandleFunc("/api/command", s.handleCommand)
	s.mux.HandleFunc("/api/tell/", s.handleTell)
	s.mux.HandleFunc("/api/events", s.handleSSE)
	s.mux.HandleFunc("/icon.svg", s.handleIcon)
	s.mux.HandleFunc("/", s.handleStatic)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers on all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Serve specific known files at root level
	switch path {
	case "/":
		path = "/index.html"
	case "/sw.js", "/manifest.json":
		// serve as-is
	}

	// Try to open the file from embedded FS
	f, err := s.staticFS.Open(path)
	if err != nil {
		// Fallback to index.html for SPA routing
		f, err = s.staticFS.Open("/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Set content type based on extension
	ext := filepath.Ext(path)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	// Cache static assets (not HTML)
	if ext != ".html" {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}

	http.ServeContent(w, r, path, stat.ModTime(), f.(readSeeker))
}

// readSeeker combines io.ReadSeeker — embedded files implement this.
type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write([]byte(iconSVG))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type statusResult struct {
		name   string
		output string
		err    error
	}

	ch := make(chan statusResult, 2)

	go func() {
		out, err := runBashCommand("check")
		ch <- statusResult{"check", out, err}
	}()
	go func() {
		out, err := runBashCommand("who")
		ch <- statusResult{"who", out, err}
	}()

	results := make(map[string]interface{})
	for i := 0; i < 2; i++ {
		res := <-ch
		if res.err != nil {
			results[res.name] = ""
		} else {
			results[res.name] = res.output
		}
	}

	results["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	writeJSON(w, results)
}

func (s *Server) handleSwarms(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	swarms, err := s.readAllSwarmsWithAgents()
	if err != nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, swarms)
}

func (s *Server) handleSwarmDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/swarm/")
	if id == "" {
		http.Error(w, "swarm id required", http.StatusBadRequest)
		return
	}

	swarm, err := s.readSwarmRaw(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Read agents
	agents, _ := s.readSwarmAgents(id)
	agentsWithNato := make([]AgentState, len(agents))
	for i, a := range agents {
		agentsWithNato[i] = a
		agentsWithNato[i].NATO = natoReverse[a.ID]
		agentsWithNato[i].SwarmID = id
	}
	swarm["agents"] = agentsWithNato

	// Read mailbox (last 20 messages)
	mailbox := s.readMailbox(id, 20)
	swarm["mailbox"] = mailbox

	writeJSON(w, swarm)
}

func (s *Server) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/agent/")
	name = strings.ToLower(name)

	agentID, ok := natoNames[name]
	if !ok {
		if strings.HasPrefix(name, "agent-") {
			agentID = name
		} else {
			http.Error(w, "unknown agent name", http.StatusBadRequest)
			return
		}
	}

	// Search all swarms for this agent
	swarmsDir := filepath.Join(s.stateDir, "swarms")
	entries, err := os.ReadDir(swarmsDir)
	if err != nil {
		http.Error(w, "no swarms directory", http.StatusNotFound)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agent, err := s.readAgent(entry.Name(), agentID)
		if err != nil {
			continue
		}
		result := map[string]interface{}{
			"id":        agent.ID,
			"pane_id":   agent.PaneID,
			"role":      agent.Role,
			"status":    agent.Status,
			"branch":    agent.Branch,
			"started":   agent.Started,
			"command":   agent.Command,
			"_nato":     natoReverse[agentID],
			"_swarm_id": entry.Name(),
		}
		writeJSON(w, result)
		return
	}

	http.Error(w, "agent not found in any swarm", http.StatusNotFound)
}

func (s *Server) handlePaneCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/pane/")
	name = strings.ToLower(name)

	agentID, ok := natoNames[name]
	if !ok {
		if strings.HasPrefix(name, "agent-") {
			agentID = name
		} else {
			writeJSON(w, map[string]string{"error": fmt.Sprintf("unknown agent name: %s", name)})
			return
		}
	}

	// Find pane_id from agent state files
	var paneID string
	swarmsDir := filepath.Join(s.stateDir, "swarms")
	entries, _ := os.ReadDir(swarmsDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agent, err := s.readAgent(entry.Name(), agentID)
		if err != nil {
			continue
		}
		if agent.PaneID != "" {
			paneID = agent.PaneID
			break
		}
	}

	if paneID == "" {
		// Fallback: try to find by listing tmux panes
		out, err := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id}:#{pane_title}").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, name) || strings.Contains(lower, agentID) {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) > 0 {
						paneID = parts[0]
						break
					}
				}
			}
		}
	}

	if paneID == "" {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("no pane found for agent %s", name)})
		return
	}

	out, err := exec.Command("tmux", "capture-pane", "-t", paneID, "-p", "-S", "-50").Output()
	if err != nil {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("failed to capture pane: %s", err.Error())})
		return
	}

	writeJSON(w, map[string]interface{}{
		"agent":   name,
		"pane_id": paneID,
		"raw":     string(out),
	})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CommandResponse{OK: false, Error: "invalid request body"})
		return
	}

	if req.Cmd == "" {
		writeJSON(w, CommandResponse{OK: false, Error: "cmd is required"})
		return
	}

	// Validate against allowlist
	fields := strings.Fields(req.Cmd)
	if len(fields) == 0 {
		writeJSON(w, CommandResponse{OK: false, Error: "empty command"})
		return
	}
	baseCmd := fields[0]
	if !allowedCommands[baseCmd] {
		writeJSON(w, CommandResponse{OK: false, Error: fmt.Sprintf("command %q not allowed. Allowed: check, who, vibe, tell, scan, swarm, review, recap, focus, ship", baseCmd)})
		return
	}

	output, err := runBashCommand(req.Cmd)

	success := true
	resp := CommandResponse{OK: true, Output: output}
	if err != nil {
		success = false
		resp = CommandResponse{OK: false, Output: output, Error: err.Error()}
	}

	// Emit SSE event for command execution
	s.broker.Publish(SSEEvent{
		Type:    "command_executed",
		Command: req.Cmd,
		Success: &success,
	})

	writeJSON(w, resp)
}

func (s *Server) handleTell(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/tell/")
	name = strings.ToLower(name)

	if _, ok := natoNames[name]; !ok {
		writeJSON(w, CommandResponse{OK: false, Error: fmt.Sprintf("unknown agent name %q", name)})
		return
	}

	var req TellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CommandResponse{OK: false, Error: "invalid request body"})
		return
	}

	if req.Msg == "" {
		writeJSON(w, CommandResponse{OK: false, Error: "msg is required"})
		return
	}

	// Sanitize the message to prevent injection
	sanitized := strings.ReplaceAll(req.Msg, "'", "'\\''")
	cmd := fmt.Sprintf("tell %s '%s'", name, sanitized)

	output, err := runBashCommand(cmd)

	success := true
	resp := CommandResponse{OK: true, Output: output}
	if err != nil {
		success = false
		resp = CommandResponse{OK: false, Output: output, Error: err.Error()}
	}

	s.broker.Publish(SSEEvent{
		Type:      "message_sent",
		AgentName: name,
		Message:   req.Msg,
		Success:   &success,
	})

	writeJSON(w, resp)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Subscribe and get history for replay
	ch, history := s.broker.Subscribe()
	defer s.broker.Unsubscribe(ch)

	// Send initial connected event
	connEvt := SSEEvent{Type: "connected", Timestamp: time.Now().UTC().Format(time.RFC3339)}
	connData, _ := json.Marshal(connEvt)
	fmt.Fprintf(w, "data: %s\n\n", connData)
	flusher.Flush()

	// Replay recent event history
	for _, evt := range history {
		data, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			// Keepalive
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// ---------------------------------------------------------------------------
// State reading
// ---------------------------------------------------------------------------

func (s *Server) readAllSwarmsWithAgents() ([]map[string]interface{}, error) {
	swarmsDir := filepath.Join(s.stateDir, "swarms")
	entries, err := os.ReadDir(swarmsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}

	var swarms []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		swarmID := entry.Name()
		sw, err := s.readSwarmRaw(swarmID)
		if err != nil {
			continue
		}

		// Read agents inline
		agents, _ := s.readSwarmAgents(swarmID)
		agentList := make([]map[string]interface{}, 0, len(agents))
		for _, a := range agents {
			agentMap := map[string]interface{}{
				"id":      a.ID,
				"pane_id": a.PaneID,
				"role":    a.Role,
				"status":  a.Status,
				"branch":  a.Branch,
				"started": a.Started,
				"_nato":   natoReverse[a.ID],
			}
			agentList = append(agentList, agentMap)
		}
		sw["agents"] = agentList
		sw["_id"] = swarmID

		swarms = append(swarms, sw)
	}
	return swarms, nil
}

func (s *Server) readSwarmRaw(id string) (map[string]interface{}, error) {
	path := filepath.Join(s.stateDir, "swarms", id, "swarm.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sw map[string]interface{}
	if err := json.Unmarshal(data, &sw); err != nil {
		return nil, err
	}
	return sw, nil
}

func (s *Server) readAgent(swarmID, agentID string) (AgentState, error) {
	path := filepath.Join(s.stateDir, "swarms", swarmID, "agents", agentID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return AgentState{}, err
	}
	var agent AgentState
	if err := json.Unmarshal(data, &agent); err != nil {
		return AgentState{}, err
	}
	return agent, nil
}

func (s *Server) readSwarmAgents(swarmID string) ([]AgentState, error) {
	agentsDir := filepath.Join(s.stateDir, "swarms", swarmID, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, err
	}

	var agents []AgentState
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		agentID := strings.TrimSuffix(entry.Name(), ".json")
		agent, err := s.readAgent(swarmID, agentID)
		if err != nil {
			continue
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func (s *Server) readMailbox(swarmID string, limit int) []MailMsg {
	mailboxDir := filepath.Join(s.stateDir, "swarms", swarmID, "mailbox")
	entries, err := os.ReadDir(mailboxDir)
	if err != nil {
		return nil
	}

	// Filter to .json files and sort by name
	var jsonFiles []os.DirEntry
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e)
		}
	}
	sort.Slice(jsonFiles, func(i, j int) bool {
		return jsonFiles[i].Name() < jsonFiles[j].Name()
	})

	// Take last N
	start := 0
	if len(jsonFiles) > limit {
		start = len(jsonFiles) - limit
	}
	jsonFiles = jsonFiles[start:]

	var messages []MailMsg
	for _, f := range jsonFiles {
		data, err := os.ReadFile(filepath.Join(mailboxDir, f.Name()))
		if err != nil {
			continue
		}
		var msg MailMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages
}

// ---------------------------------------------------------------------------
// File watcher (poll-based, 1-second interval)
// ---------------------------------------------------------------------------

func (s *Server) watchStateDir(ctx context.Context) {
	swarmsDir := filepath.Join(s.stateDir, "swarms")
	os.MkdirAll(swarmsDir, 0755)

	knownState := make(map[string]string) // path -> modKey
	var mu sync.Mutex

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			s.scanForChanges(swarmsDir, knownState)
			mu.Unlock()
		}
	}
}

func (s *Server) scanForChanges(swarmsDir string, knownState map[string]string) {
	currentFiles := make(map[string]bool)

	filepath.WalkDir(swarmsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		currentFiles[path] = true

		info, err := d.Info()
		if err != nil {
			return nil
		}

		modKey := fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano())
		prev, exists := knownState[path]

		if !exists {
			knownState[path] = modKey
			s.emitFileEvent(path, "created")
		} else if prev != modKey {
			knownState[path] = modKey
			s.emitFileEvent(path, "modified")
		}

		return nil
	})

	// Detect deletions
	for path := range knownState {
		if !currentFiles[path] {
			delete(knownState, path)
			s.emitFileEvent(path, "deleted")
		}
	}
}

func (s *Server) emitFileEvent(path, changeType string) {
	rel, err := filepath.Rel(filepath.Join(s.stateDir, "swarms"), path)
	if err != nil {
		return
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return
	}

	swarmID := parts[0]

	if parts[1] == "swarm.json" {
		eventType := "swarm_updated"
		status := ""
		name := ""
		if changeType == "created" {
			eventType = "swarm_created"
		} else if changeType == "deleted" {
			eventType = "swarm_deleted"
		}

		if changeType != "deleted" {
			sw, err := s.readSwarmRaw(swarmID)
			if err == nil {
				if s, ok := sw["status"].(string); ok {
					status = s
					if s == "killed" {
						eventType = "swarm_killed"
					}
				}
				if n, ok := sw["name"].(string); ok {
					name = n
				}
			}
		}

		s.broker.Publish(SSEEvent{
			Type:    eventType,
			SwarmID: swarmID,
			Status:  status,
			Name:    name,
		})

	} else if len(parts) == 3 && parts[1] == "agents" && strings.HasSuffix(parts[2], ".json") {
		agentID := strings.TrimSuffix(parts[2], ".json")
		natoName := natoReverse[agentID]

		if changeType == "deleted" {
			s.broker.Publish(SSEEvent{
				Type:      "agent_removed",
				SwarmID:   swarmID,
				AgentID:   agentID,
				AgentName: natoName,
			})
			return
		}

		agent, err := s.readAgent(swarmID, agentID)
		if err != nil {
			return
		}

		eventType := "agent_updated"
		switch {
		case agent.Status == "completed" || agent.Status == "done":
			eventType = "agent_completed"
		case agent.Status == "error" || agent.Status == "failed":
			eventType = "agent_error"
		case changeType == "created":
			eventType = "agent_started"
		}

		s.broker.Publish(SSEEvent{
			Type:      eventType,
			SwarmID:   swarmID,
			AgentID:   agentID,
			AgentName: natoName,
			Status:    agent.Status,
			Role:      agent.Role,
			Branch:    agent.Branch,
		})

	} else if len(parts) >= 3 && parts[1] == "mailbox" {
		s.broker.Publish(SSEEvent{
			Type:    "message_sent",
			SwarmID: swarmID,
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runBashCommand(cmd string) (string, error) {
	bashCmd := fmt.Sprintf("source ~/.bashrc 2>/dev/null; source ~/.zshrc 2>/dev/null; %s", cmd)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "bash", "-c", bashCmd)
	c.Env = os.Environ()

	output, err := c.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	stateDir := os.Getenv("OMACMUX_STATE_DIR")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("cannot determine home directory:", err)
		}
		stateDir = filepath.Join(home, ".local", "share", "omacmux")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8384"
	}

	srv := NewServer(stateDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start state watcher
	go srv.watchStateDir(ctx)

	httpSrv := &http.Server{
		Addr:    ":" + port,
		Handler: srv,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
		httpSrv.Shutdown(context.Background())
	}()

	hostname, _ := os.Hostname()
	log.Printf("omacmux remote server starting on :%s", port)
	log.Printf("state dir: %s", stateDir)
	log.Printf("hostname: %s", hostname)
	log.Printf("dashboard: http://localhost:%s/", port)

	// Print initial state
	swarms, err := srv.readAllSwarmsWithAgents()
	if err == nil && len(swarms) > 0 {
		log.Printf("found %d swarm(s) in state dir", len(swarms))
		for _, sw := range swarms {
			id, _ := sw["id"].(string)
			topo, _ := sw["topology"].(string)
			status, _ := sw["status"].(string)
			agents, _ := sw["agents"].([]map[string]interface{})
			log.Printf("  %s (%s) - %s - %d agents", id, topo, status, len(agents))
		}
	} else {
		log.Printf("no active swarms found")
	}

	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Println("server stopped")
}
