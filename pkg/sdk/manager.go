package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/session/database"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ListFilesArgs struct { Dir string `json:"dir"` }
type ListFilesResult struct { Files []string `json:"files"`; Error string `json:"error,omitempty"` }
func listLocalFiles(ctx tool.Context, args ListFilesArgs) (ListFilesResult, error) {
	if args.Dir == "" { args.Dir = "." }
	entries, err := os.ReadDir(args.Dir); if err != nil { return ListFilesResult{Error: err.Error()}, nil }
	var files []string
	for _, entry := range entries {
		name := entry.Name(); if entry.IsDir() { name += "/" }; files = append(files, name)
	}
	return ListFilesResult{Files: files}, nil
}

type ReadFileArgs struct { Path string `json:"path"` }
type ReadFileResult struct { Content string `json:"content"`; Error string `json:"error,omitempty"` }
func readLocalFile(ctx tool.Context, args ReadFileArgs) (ReadFileResult, error) {
	b, err := os.ReadFile(args.Path); if err != nil { return ReadFileResult{Error: err.Error()}, nil }
	return ReadFileResult{Content: string(b)}, nil
}

type GrepArgs struct { Pattern string `json:"pattern"`; Dir string `json:"dir"` }
type GrepResult struct { Matches []string `json:"matches"`; Error string `json:"error,omitempty"` }
func grepSearch(ctx tool.Context, args GrepArgs) (GrepResult, error) {
	if args.Dir == "" { args.Dir = "." }
	cmd := exec.Command("grep", "-r", "-l", args.Pattern, args.Dir)
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok { return GrepResult{Matches: []string{}}, nil }
		return GrepResult{Error: err.Error()}, nil
	}
	matches := strings.Split(strings.TrimSpace(string(out)), "\n")
	return GrepResult{Matches: matches}, nil
}

type WriteFileArgs struct { Path string `json:"path"`; Content string `json:"content"` }
type WriteFileResult struct { Success bool `json:"success"`; Error string `json:"error,omitempty"` }
func writeLocalFile(ctx tool.Context, args WriteFileArgs) (WriteFileResult, error) {
	dir := filepath.Dir(args.Path); if err := os.MkdirAll(dir, 0755); err != nil { return WriteFileResult{Success: false, Error: err.Error()}, nil }
	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil { return WriteFileResult{Success: false, Error: err.Error()}, nil }
	return WriteFileResult{Success: true}, nil
}

type WebFetchArgs struct { URL string `json:"url"` }
type WebFetchResult struct { Content string `json:"content"`; Error string `json:"error,omitempty"` }
func webFetch(ctx tool.Context, args WebFetchArgs) (WebFetchResult, error) {
	resp, err := http.Get(args.URL); if err != nil { return WebFetchResult{Error: err.Error()}, nil }
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body); if err != nil { return WebFetchResult{Error: err.Error()}, nil }
	return WebFetchResult{Content: string(b)}, nil
}

type GoogleSearchArgs struct { Query string `json:"query"` }
type GoogleSearchResult struct { Response string `json:"response"`; Error string `json:"error,omitempty"` }
func googleSearchFunc(ctx tool.Context, args GoogleSearchArgs) (GoogleSearchResult, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY"); if apiKey == "" { return GoogleSearchResult{Error: "GOOGLE_API_KEY is not set"}, nil }
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey}); if err != nil { return GoogleSearchResult{Error: err.Error()}, nil }
	resp, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash", genai.Text(args.Query), &genai.GenerateContentConfig{Tools: []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}}); if err != nil { return GoogleSearchResult{Error: err.Error()}, nil }
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 { return GoogleSearchResult{Response: resp.Candidates[0].Content.Parts[0].Text}, nil }
	return GoogleSearchResult{Response: "No results found"}, nil
}

type AgentManifest struct {
	Name string `yaml:"name"`; Framework string `yaml:"framework"`; Language string `yaml:"language"`
	Entrypoint string `yaml:"entrypoint,omitempty"`; Description string `yaml:"description,omitempty"`; Tools []string `yaml:"tools,omitempty"`
}

type SessionInfo struct { ID string; UpdatedAt string; Summary string }
type ModelInfo struct { Name string; DisplayName string; Description string; Version string }

type AgentManager interface {
	Discover(ctx context.Context, dir string) (*AgentManifest, error)
	AddContext(path string) error; DropContext(path string); ListContext() []string
	Plan(ctx context.Context, prompt string) (*ExecutionGraph, error)
	ExecuteGraph(ctx context.Context, g *ExecutionGraph) (<-chan ChatEvent, *Orchestrator, error)
	Chat(ctx context.Context, prompt string) (<-chan ChatEvent, error)
	Reset(); Reload() error; Rewind(n int) error; Skills() []*Skill
	ListModels(ctx context.Context) ([]ModelInfo, error); ListSessions(ctx context.Context) ([]SessionInfo, error)
	SetDebug(enabled bool); IsDebug() bool
}

type telemetryContextKey struct{}

type defaultManager struct {
	run *runner.Runner; db *gorm.DB; sessionSvc session.Service; userID string; sessionID string; skills []*Skill; clientCfg *genai.ClientConfig; pinnedContext map[string]string; inputInstruction string; debugMode bool
	flashModel model.LLM; proModel model.LLM; fastModel model.LLM; toolRegistry map[string]tool.Tool; inputAgent agent.Agent; subAgentNames []string; agents map[string]agent.Agent
}

type ManagerConfig struct { Model model.LLM; ResumeLastSession bool }

func NewManager(cfg ...ManagerConfig) (AgentManager, error) {
	ctx := context.Background(); var flashModel, proModel, fastModel model.LLM; clientConfig := &genai.ClientConfig{}
	if len(cfg) > 0 && cfg[0].Model != nil { flashModel = cfg[0].Model; proModel = cfg[0].Model; fastModel = cfg[0].Model } else {
		if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" { clientConfig.APIKey = apiKey }
		userCfg, _ := LoadConfig(); proModelName := "gemini-3.1-pro-preview"; if userCfg != nil && userCfg.Model != "" && userCfg.Model != "auto" { proModelName = userCfg.Model }
		var err error
		flashModel, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig); if err != nil { return nil, err }
		proModel, err = gemini.NewModel(ctx, proModelName, clientConfig); if err != nil { return nil, err }
		fastModel, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig); if err != nil { return nil, err }
	}
	listTool, _ := functiontool.New(functiontool.Config{Name: "list_local_files"}, listLocalFiles)
	readTool, _ := functiontool.New(functiontool.Config{Name: "read_local_file"}, readLocalFile)
	grepTool, _ := functiontool.New(functiontool.Config{Name: "grep_search"}, grepSearch)
	writeTool, _ := functiontool.New(functiontool.Config{Name: "write_local_file"}, writeLocalFile)
	gitCommit, _ := functiontool.New(functiontool.Config{Name: "git_commit"}, gitCommitTool)
	gitPush, _ := functiontool.New(functiontool.Config{Name: "git_push"}, gitPushTool)
	bashExecute, _ := functiontool.New(functiontool.Config{Name: "bash_execute"}, bashExecuteTool)
	webFetchTool, _ := functiontool.New(functiontool.Config{Name: "web_fetch"}, webFetch)
	googleSearchTool, _ := functiontool.New(functiontool.Config{Name: "google_search"}, googleSearchFunc)
	replanTool, _ := functiontool.New(functiontool.Config{Name: "request_replan"}, requestReplan)
	toolRegistry := map[string]tool.Tool{
		"list_local_files": listTool, "read_local_file": readTool, "grep_search": grepTool, "write_local_file": writeTool, "git_commit": gitCommit, "git_push": gitPush, "bash_execute": bashExecute, "web_fetch": webFetchTool, "google_search": googleSearchTool, "request_replan": replanTool,
	}
	home, _ := os.UserHomeDir(); dbDir := filepath.Join(home, ".config", "swarm"); _ = os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, "sessions.db"); dialector := sqlite.Open(dbPath); gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(dialector, gormCfg); if err != nil { return nil, err }
	sessionSvc, err := database.NewSessionService(dialector, gormCfg); if err != nil { return nil, err }
	_ = database.AutoMigrate(sessionSvc)
	sessionID := ""; if len(cfg) > 0 && cfg[0].ResumeLastSession {
		resp, err := sessionSvc.List(ctx, &session.ListRequest{AppName: "swarm-cli", UserID: "local_user"})
		if err == nil && len(resp.Sessions) > 0 {
			var lastSession session.Session; var lastTime int64
			for _, s := range resp.Sessions { if s.LastUpdateTime().UnixNano() > lastTime { lastTime = s.LastUpdateTime().UnixNano(); lastSession = s } }
			if lastSession != nil { sessionID = lastSession.ID() }
		}
	}
	if sessionID == "" { sessionID = fmt.Sprintf("session_%d", rand.Int63()) }
	_, _ = sessionSvc.Create(ctx, &session.CreateRequest{AppName: "swarm-cli", UserID: "local_user", SessionID: sessionID})
	m := &defaultManager{
		db: db, sessionSvc: sessionSvc, userID: "local_user", sessionID: sessionID, clientCfg: clientConfig, pinnedContext: make(map[string]string), flashModel: flashModel, proModel: proModel, fastModel: fastModel, toolRegistry: toolRegistry,
	}
	if err := m.Reload(); err != nil { return nil, err }
	return m, nil
}

type MemorySessionService struct { mu sync.RWMutex; sessions map[string]*memSession }
type memSession struct { id string; events []*session.Event; state map[string]any; updatedAt time.Time }
type memEvents []*session.Event
func (e memEvents) All() iter.Seq[*session.Event] { return func(yield func(*session.Event) bool) { for _, ev := range e { if !yield(ev) { return } } } }
func (e memEvents) Len() int { return len(e) }
func (e memEvents) At(i int) *session.Event { return e[i] }
type memState struct { mu *sync.RWMutex; data map[string]any }
func (s memState) Get(k string) (any, error) { s.mu.RLock(); defer s.mu.RUnlock(); v, ok := s.data[k]; if !ok { return nil, session.ErrStateKeyNotExist }; return v, nil }
func (s memState) Set(k string, v any) error { s.mu.Lock(); defer s.mu.Unlock(); s.data[k] = v; return nil }
func (s memState) All() iter.Seq2[string, any] { return func(yield func(string, any) bool) { s.mu.RLock(); defer s.mu.RUnlock(); for k, v := range s.data { if !yield(k, v) { return } } } }
func (s *memSession) ID() string { return s.id }
func (s *memSession) AppName() string { return "swarm-cli" }
func (s *memSession) UserID() string { return "local_user" }
func (s *memSession) LastUpdateTime() time.Time { return s.updatedAt }
func (s *memSession) Events() session.Events { return memEvents(s.events) }
func (s *memSession) State() session.State { return memState{mu: &sync.RWMutex{}, data: s.state} }
func (s *memSession) Metadata() map[string]string { return nil }
func NewMemorySessionService() *MemorySessionService { return &MemorySessionService{sessions: make(map[string]*memSession)} }
func (s *MemorySessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	s.mu.Lock(); defer s.mu.Unlock(); sess := &memSession{id: req.SessionID, state: make(map[string]any), updatedAt: time.Now()}; s.sessions[req.SessionID] = sess; return &session.CreateResponse{Session: sess}, nil
}
func (s *MemorySessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	s.mu.RLock(); defer s.mu.RUnlock(); sess, ok := s.sessions[req.SessionID]; if !ok { return nil, fmt.Errorf("not found") }; return &session.GetResponse{Session: sess}, nil
}
func (s *MemorySessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	s.mu.Lock(); defer s.mu.Unlock(); ms, ok := s.sessions[sess.ID()]; if !ok { return fmt.Errorf("not found") }; ms.events = append(ms.events, event); ms.updatedAt = time.Now(); return nil
}
func (s *MemorySessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) { return &session.ListResponse{}, nil }
func (s *MemorySessionService) Delete(ctx context.Context, req *session.DeleteRequest) error { s.mu.Lock(); defer s.mu.Unlock(); delete(s.sessions, req.SessionID); return nil }

func (m *defaultManager) IsDebug() bool { return m.debugMode }
func (m *defaultManager) SetDebug(enabled bool) { m.debugMode = enabled }
func (m *defaultManager) Reload() error {
	var subAgents []agent.Agent; var loadedSkills []*Skill; skillDirs := []string{}
	entries, err := os.ReadDir("skills")
	if err == nil { for _, entry := range entries { if entry.IsDir() { skillDirs = append(skillDirs, filepath.Join("skills", entry.Name())) } } }
	for _, dir := range skillDirs {
		skill, err := LoadSkill(dir); if err != nil { continue }
		loadedSkills = append(loadedSkills, skill); var skillTools []tool.Tool; skillTools = append(skillTools, m.toolRegistry["request_replan"])
		for _, toolName := range skill.Manifest.Tools { if t, ok := m.toolRegistry[toolName]; ok { skillTools = append(skillTools, t) } }
		targetModel := m.proModel; if skill.Manifest.Model == "flash" { targetModel = m.flashModel }
		skillAgent, err := llmagent.New(llmagent.Config{
			Name: skill.Manifest.Name, Model: targetModel, Description: skill.Manifest.Description,
			Instruction: skill.Instructions, Tools: skillTools,
		})
		if err != nil { return err }
		subAgents = append(subAgents, skillAgent)
	}
	var subAgentNames []string; for _, sa := range subAgents { subAgentNames = append(subAgentNames, sa.Name()) }
	swarmInstruction := fmt.Sprintf("You are the primary Swarm Agent. Help the user build, test, and deploy AI agents. NODE AUTONOMY: You are responsible for doing your best to fulfill the request. You can respond with output directly, or orchestrate sub-tasks yourself. You can call upon the Planning Agent if you require a structured execution graph. Available sub-agents: %s", strings.Join(subAgentNames, ", "))
	swarmAgent, err := llmagent.New(llmagent.Config{
		Name: "swarm_agent", Model: m.flashModel, Instruction: swarmInstruction,
		Tools: []tool.Tool{m.toolRegistry["list_local_files"], m.toolRegistry["read_local_file"], m.toolRegistry["grep_search"]},
		SubAgents: subAgents,
	})
	if err != nil { return err }
	r, err := runner.New(runner.Config{AppName: "swarm-cli", Agent: swarmAgent, SessionService: m.sessionSvc})
	if err != nil { return err }
	inputInstruction := fmt.Sprintf("You are the Input Agent. Invisible. Classify intent. Sub-agents: %s. RESPONSE: [reply] for social. ROUTE TO: [agent].", strings.Join(subAgentNames, ", "))
	inputAgent, err := llmagent.New(llmagent.Config{Name: "input_agent", Model: m.flashModel, Instruction: inputInstruction})
	if err != nil { return err }
	m.run = r; m.skills = loadedSkills; m.subAgentNames = subAgentNames; m.inputInstruction = inputInstruction; m.inputAgent = inputAgent
	m.agents = make(map[string]agent.Agent); m.agents[swarmAgent.Name()] = swarmAgent
	for _, sa := range subAgents { m.agents[sa.Name()] = sa }
	return nil
}

func (m *defaultManager) Skills() []*Skill { return m.skills }
func (m *defaultManager) AddContext(path string) error { b, err := os.ReadFile(path); if err != nil { return err }; m.pinnedContext[path] = string(b); return nil }
func (m *defaultManager) DropContext(path string) { if path == "all" { m.pinnedContext = make(map[string]string) } else { delete(m.pinnedContext, path) } }
func (m *defaultManager) ListContext() []string { var p []string; for path := range m.pinnedContext { p = append(p, path) }; return p }
func (m *defaultManager) Reset() { m.sessionID = fmt.Sprintf("session_%d", rand.Int63()) }
func (m *defaultManager) ListModels(ctx context.Context) ([]ModelInfo, error) {
	client, err := genai.NewClient(ctx, m.clientCfg); if err != nil { return nil, err }
	var models []ModelInfo; iter := client.Models.All(ctx)
	for mo, err := range iter { if err != nil { return nil, err }; name := strings.TrimPrefix(mo.Name, "models/"); models = append(models, ModelInfo{Name: name, DisplayName: mo.DisplayName, Description: mo.Description, Version: mo.Version}) }
	return models, nil
}

type ChatEventType string
const (
	ChatEventHandoff ChatEventType = "handoff"; ChatEventToolCall ChatEventType = "tool_call"; ChatEventToolResult ChatEventType = "tool_result"
	ChatEventTelemetry ChatEventType = "telemetry"; ChatEventThought ChatEventType = "thought"; ChatEventObserver ChatEventType = "observer"
	ChatEventReplan ChatEventType = "replan"; ChatEventDebug ChatEventType = "debug"; ChatEventFinalResponse ChatEventType = "final_response"; ChatEventError ChatEventType = "error"
)
type ChatEvent struct { Type ChatEventType; Agent, TaskID, Content string }

func (m *defaultManager) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	resp, err := m.sessionSvc.List(ctx, &session.ListRequest{AppName: "swarm-cli", UserID: m.userID}); if err != nil { return nil, err }
	var infos []SessionInfo
	for _, s := range resp.Sessions {
		summary := s.ID(); var event struct{ Content string }
		err := m.db.Table("events").Select("content").Where("session_id = ? AND author = ?", s.ID(), "user").Order("timestamp DESC").Limit(1).Find(&event).Error
		if err == nil && event.Content != "" { summary = event.Content }; if len(summary) > 40 { summary = summary[:37] + "..." }
		infos = append(infos, SessionInfo{ID: s.ID(), UpdatedAt: s.LastUpdateTime().Format("2006-01-02 15:04:05"), Summary: summary})
	}
	return infos, nil
}

func (m *defaultManager) Plan(ctx context.Context, prompt string) (*ExecutionGraph, error) {
	systemPrompt := fmt.Sprintf("You are the Planning Agent. Decompose request into DAG JSON. TRIVIAL: use immediate_response. COMPLEX: DEEP_PLAN_REQUIRED. Agents: %s", strings.Join(m.subAgentNames, ", "))
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}, Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system"))}}, false)
	var jsonStr string; for resp, err := range respIter { if err != nil { return nil, err }; if resp.Content != nil && len(resp.Content.Parts) > 0 { jsonStr += resp.Content.Parts[0].Text } }
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "DEEP_PLAN_REQUIRED" {
		respIter = m.proModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}, Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system"))}}, false)
		jsonStr = ""; for resp, err := range respIter { if err != nil { return nil, err }; if resp.Content != nil && len(resp.Content.Parts) > 0 { jsonStr += resp.Content.Parts[0].Text } }
		jsonStr = strings.TrimSpace(jsonStr)
	}
	if strings.HasPrefix(jsonStr, "```json") { jsonStr = strings.TrimPrefix(jsonStr, "```json"); jsonStr = strings.TrimSuffix(jsonStr, "```"); jsonStr = strings.TrimSpace(jsonStr) }
	var graph ExecutionGraph; if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil { return nil, err }; return &graph, nil
}

func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan ChatEvent, error) {
	out := make(chan ChatEvent, 1000)
	go func() {
		defer close(out); swarmStartTime := time.Now(); out <- ChatEvent{Type: ChatEventThought, Agent: "Input Agent", Content: "Classifying…"}
		inputIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}, Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(m.inputInstruction, genai.Role("system"))}}, false)
		for resp, err := range inputIter {
			if err != nil { out <- ChatEvent{Type: ChatEventError, Content: err.Error()}; return }
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				t := resp.Content.Parts[0].Text
				if strings.HasPrefix(t, "RESPONSE: ") {
					r := strings.TrimPrefix(t, "RESPONSE: ")
					if m.runCOA(ctx, out, "Input Agent", r) {
						if m.debugMode { out <- ChatEvent{Type: ChatEventDebug, Agent: "Orchestrator", Content: fmt.Sprintf("Duration: %s", time.Since(swarmStartTime))} }
						out <- ChatEvent{Type: ChatEventFinalResponse, Agent: "Input Agent", Content: r}; return
					}
				}
			}
			break
		}
		out <- ChatEvent{Type: ChatEventThought, Agent: "Planning Agent", Content: "Planning…"}
		graph, err := m.Plan(ctx, prompt); if err != nil { out <- ChatEvent{Type: ChatEventError, Content: err.Error()}; return }
		if graph.ImmediateResponse != "" {
			if m.runCOA(ctx, out, "Planning Agent", graph.ImmediateResponse) {
				if m.debugMode { out <- ChatEvent{Type: ChatEventDebug, Agent: "Orchestrator", Content: fmt.Sprintf("Duration: %s", time.Since(swarmStartTime))} }
				out <- ChatEvent{Type: ChatEventFinalResponse, Agent: "Planning Agent", Content: graph.ImmediateResponse}; return
			}
		}
		events, orchestrator, err := m.ExecuteGraph(ctx, graph); if err != nil { out <- ChatEvent{Type: ChatEventError, Content: err.Error()}; return }
		for event := range events { out <- event }
		if m.debugMode { traj := orchestrator.GetTrajectory(); b, _ := json.MarshalIndent(traj, "", "  "); out <- ChatEvent{Type: ChatEventDebug, Agent: "Orchestrator", Content: string(b)} }
	}()
	return out, nil
}

func (m *defaultManager) ExecuteGraph(ctx context.Context, g *ExecutionGraph) (<-chan ChatEvent, *Orchestrator, error) {
	out := make(chan ChatEvent, 1000); o := NewOrchestrator(g); memSvc := NewMemorySessionService()
	go func() {
		defer close(out); type completedTask struct { ID, Result string; Replan bool }; completed := make(chan completedTask, 100); activeTasks := 0; var wg sync.WaitGroup
		for {
			ready := o.GetReadyTasks()
			for _, t := range ready {
				o.MarkActive(t.ID); activeTasks++; wg.Add(1)
				go func(task Task) {
					defer wg.Done(); var taskResult string; var needsReplan bool; defer func() { completed <- completedTask{ID: task.ID, Result: taskResult, Replan: needsReplan} }()
					targetAgent, ok := m.agents[task.Agent]; if !ok { out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", TaskID: task.ID, Content: "Agent not found"}; return }
					sessResp, _ := memSvc.Create(ctx, &session.CreateRequest{AppName: "swarm-cli", UserID: "local_user", SessionID: task.ID})
					taskRunner, _ := runner.New(runner.Config{AppName: "swarm-cli", Agent: targetAgent, SessionService: memSvc})
					out <- ChatEvent{Type: ChatEventThought, Agent: targetAgent.Name(), TaskID: task.ID, Content: "Starting…"}
					telemetryChan := make(chan string, 100); toolCtx, cancel := context.WithCancel(context.WithValue(ctx, telemetryContextKey{}, (chan<- string)(telemetryChan))); defer cancel(); telemetryDone := make(chan struct{}); wg.Add(1)
					go func() {
						defer wg.Done(); var lastLines []string
						for line := range telemetryChan {
							out <- ChatEvent{Type: ChatEventTelemetry, Agent: targetAgent.Name(), TaskID: task.ID, Content: line}; lastLines = append(lastLines, line)
							if len(lastLines) >= 10 {
								wg.Add(1); go func(history []string) {
									defer wg.Done(); obsCtx, _ := context.WithTimeout(ctx, 10*time.Second)
									obsPrompt := fmt.Sprintf("Monitor: %s. Telemetry: %s", targetAgent.Name(), strings.Join(history, "\n"))
									respIter := m.fastModel.GenerateContent(obsCtx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(obsPrompt, genai.Role("user"))}}, false)
									for resp, err := range respIter { if err == nil && resp.Content != nil && len(resp.Content.Parts) > 0 && strings.HasPrefix(resp.Content.Parts[0].Text, "INTERVENE:") { out <- ChatEvent{Type: ChatEventObserver, Agent: "Observer", TaskID: task.ID, Content: resp.Content.Parts[0].Text} }; break }
								}(lastLines); lastLines = nil
							}
						}
						close(telemetryDone)
					}();
					promptStr := fmt.Sprintf("Task: %s. Instructions: %s", task.Name, task.Prompt); events := taskRunner.Run(toolCtx, m.userID, sessResp.Session.ID(), genai.NewContentFromText(promptStr, genai.Role("user")), agent.RunConfig{})
					var full strings.Builder
					for event, err := range events {
						if err != nil { out <- ChatEvent{Type: ChatEventError, Agent: targetAgent.Name(), TaskID: task.ID, Content: err.Error()}; break }
						if event.Content != nil { for _, part := range event.Content.Parts { if part.FunctionCall != nil { if part.FunctionCall.Name == "request_replan" { needsReplan = true }; out <- ChatEvent{Type: ChatEventToolCall, Agent: targetAgent.Name(), TaskID: task.ID, Content: part.FunctionCall.Name} }; if part.FunctionResponse != nil { out <- ChatEvent{Type: ChatEventToolResult, Agent: targetAgent.Name(), TaskID: task.ID, Content: part.FunctionResponse.Name} }; if part.Text != "" && !part.Thought { full.WriteString(part.Text) } } }
					}
					finalText := full.String(); if m.runCOA(ctx, out, targetAgent.Name(), finalText) { out <- ChatEvent{Type: ChatEventFinalResponse, Agent: targetAgent.Name(), TaskID: task.ID, Content: finalText}; taskResult = finalText } else { needsReplan = true; taskResult = "COA rejected." }; close(telemetryChan); <-telemetryDone
				}(t)
			}
			if activeTasks == 0 { if o.IsComplete() { break } else { out <- ChatEvent{Type: ChatEventError, Content: "Deadlock"}; break } }
			select {
			case <-ctx.Done(): return
			case done := <-completed: o.MarkComplete(done.ID, done.Result); activeTasks--
				if done.Replan { newG, err := m.Plan(ctx, "Pivot: "+done.Result); if err == nil { o.AddTasks(newG.Tasks...) } }
			}
		}
		wg.Wait()
	}()
	return out, o, nil
}

func (m *defaultManager) runCOA(ctx context.Context, out chan<- ChatEvent, agentName, content string) bool {
	coaPrompt := fmt.Sprintf("Sanity check worker: %s. Response: %s. If toxic/hallucinating output FIX: [reason]. Else OK.", agentName, content)
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(coaPrompt, genai.Role("user"))}}, false)
	approved := true; for resp, err := range respIter { if err == nil && resp.Content != nil && len(resp.Content.Parts) > 0 && strings.HasPrefix(resp.Content.Parts[0].Text, "FIX:") { out <- ChatEvent{Type: ChatEventThought, Agent: "Output Agent", Content: "Rejected: " + resp.Content.Parts[0].Text}; approved = false }; break }
	return approved
}

func (m *defaultManager) Rewind(n int) error {
	if n <= 0 { return nil }
	var evs []struct{ Timestamp string }; err := m.db.Table("events").Select("timestamp").Where("session_id = ? AND author = ?", m.sessionID, "user").Order("timestamp DESC").Limit(n).Find(&evs).Error
	if err != nil { return err }; if len(evs) < n { return m.db.Table("events").Where("session_id = ?", m.sessionID).Delete(nil).Error }
	return m.db.Table("events").Where("session_id = ? AND timestamp >= ?", m.sessionID, evs[len(evs)-1].Timestamp).Delete(nil).Error
}
