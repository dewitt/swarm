package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

type SessionInfo struct {
	ID        string
	UpdatedAt string
	Summary   string
}
type ModelInfo struct {
	Name        string
	DisplayName string
	Description string
	Version     string
}

type AgentState string

const (
	AgentStatePending   AgentState = "pending"
	AgentStateSpawning  AgentState = "spawning"
	AgentStateThinking  AgentState = "thinking"
	AgentStateExecuting AgentState = "executing"
	AgentStateWaiting   AgentState = "waiting" // Blocked on HITL
	AgentStateComplete  AgentState = "complete"
	AgentStateError     AgentState = "error"
)

type ObservableEvent struct {
	Timestamp time.Time
	AgentName string
	SpanID    string

	// Topology context
	TaskName string
	ParentID string

	// Core State Update
	State AgentState

	// Execution Context
	ToolName string
	ToolArgs map[string]any

	// Output
	Thought      string
	FinalContent string
	Error        error

	// Observers
	ObserverSummary string
}

type Swarm interface {
	AddContext(path string) error
	DropContext(path string)
	ListContext() []string
	Plan(ctx context.Context, prompt string) (*ExecutionGraph, error)
	Execute(ctx context.Context, g *ExecutionGraph, o *Engine) (<-chan ObservableEvent, *Engine, error)
	Chat(ctx context.Context, prompt string) (<-chan ObservableEvent, error)
	Reset()
	Reload() error
	Rewind(n int) error
	Skills() []*Skill
	ListModels(ctx context.Context) ([]ModelInfo, error)
	ListSessions(ctx context.Context) ([]SessionInfo, error)
	SetDebug(enabled bool)
	IsDebug() bool
	Explain(ctx context.Context, traj Trajectory) (string, error)
}

type telemetryContextKey struct{}

type defaultSwarm struct {
	run                 *runner.Runner
	db                  *gorm.DB
	sessionSvc          session.Service
	userID              string
	sessionID           string
	skills              []*Skill
	clientCfg           *genai.ClientConfig
	pinnedContext       map[string]string
	inputInstruction    string
	outputInstruction   string
	planningInstruction string
	debugMode           bool
	flashModel          model.LLM
	proModel            model.LLM
	fastModel           model.LLM
	toolRegistry        map[string]tool.Tool
	inputAgent          agent.Agent
	subAgentNames       []string
	agents              map[string]agent.Agent
	lastAgent           string // Tracks the last agent to respond
	activeEngine        *Engine // The currently executing Engine, used for dynamic task mutability
	outChan             chan<- ObservableEvent
}

type SwarmConfig struct {
	Model             model.LLM
	ResumeLastSession bool
	Debug             bool
}

func NewSwarm(cfg ...SwarmConfig) (Swarm, error) {
	ctx := context.Background()
	debugMode := false
	if len(cfg) > 0 && cfg[0].Debug {
		debugMode = true
	}
	var flashModel, proModel, fastModel model.LLM
	clientConfig := &genai.ClientConfig{}
	if len(cfg) > 0 && cfg[0].Model != nil {
		flashModel = cfg[0].Model
		proModel = cfg[0].Model
		fastModel = cfg[0].Model
	} else {
		if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
			clientConfig.APIKey = apiKey
		}

		// Configure a robust, fast-failing HTTP client to prevent network deadlocks
		// (e.g., HTTP/2 silent connection drops or endless flow waiting).
		clientConfig.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second, // Max 15s wait for first byte (fail fast)
			},
		}

		userCfg, _ := LoadConfig()
		proModelName := "gemini-3.1-pro-preview"
		if userCfg != nil && userCfg.Model != "" && userCfg.Model != "auto" {
			proModelName = userCfg.Model
		}
		var err error
		flashModel, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig)
		if err != nil {
			return nil, err
		}
		proModel, err = gemini.NewModel(ctx, proModelName, clientConfig)
		if err != nil {
			return nil, err
		}
		fastModel, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig)
		if err != nil {
			return nil, err
		}

		// Wrap models with a forced, hard timeout per request to prevent hanging on writeRequest
		flashModel = &timeoutModel{underlying: flashModel, timeout: 30 * time.Second}
		proModel = &timeoutModel{underlying: proModel, timeout: 60 * time.Second}
		fastModel = &timeoutModel{underlying: fastModel, timeout: 30 * time.Second}
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

	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".config", "swarm")
	_ = os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, "sessions.db")
	dialector := sqlite.Open(dbPath)
	gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}
	sessionSvc, err := database.NewSessionService(dialector, gormCfg)
	if err != nil {
		return nil, err
	}
	_ = database.AutoMigrate(sessionSvc)
	sessionID := ""
	if len(cfg) > 0 && cfg[0].ResumeLastSession {
		resp, err := sessionSvc.List(ctx, &session.ListRequest{AppName: "swarm-cli", UserID: "local_user"})
		if err == nil && len(resp.Sessions) > 0 {
			var lastSession session.Session
			var lastTime int64
			for _, s := range resp.Sessions {
				if s.LastUpdateTime().UnixNano() > lastTime {
					lastTime = s.LastUpdateTime().UnixNano()
					lastSession = s
				}
			}
			if lastSession != nil {
				sessionID = lastSession.ID()
			}
		}
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", rand.Int63())
	}
	_, _ = sessionSvc.Create(ctx, &session.CreateRequest{AppName: "swarm-cli", UserID: "local_user", SessionID: sessionID})

	m := &defaultSwarm{
		db: db, sessionSvc: sessionSvc, userID: "local_user", sessionID: sessionID, clientCfg: clientConfig, pinnedContext: make(map[string]string), flashModel: flashModel, proModel: proModel, fastModel: fastModel, debugMode: debugMode,
	}

	readStateTool, _ := functiontool.New(functiontool.Config{Name: "read_state"}, m.readState)
	writeStateTool, _ := functiontool.New(functiontool.Config{Name: "write_state"}, m.writeState)
	spawnSubtaskTool, _ := functiontool.New(functiontool.Config{Name: "spawn_subtask"}, m.spawnSubtask)
	m.toolRegistry = map[string]tool.Tool{
		"list_local_files": listTool, "read_local_file": readTool, "grep_search": grepTool, "write_local_file": writeTool, "git_commit": gitCommit, "git_push": gitPush, "bash_execute": bashExecute, "web_fetch": webFetchTool, "google_search": googleSearchTool, "request_replan": replanTool,
		"read_state": readStateTool, "write_state": writeStateTool, "spawn_subtask": spawnSubtaskTool,
	}

	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *defaultSwarm) readState(ctx tool.Context, req struct{ Key string }) (string, error) {
	resp, err := m.sessionSvc.Get(context.Background(), &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID})
	if err != nil {
		return "", err
	}
	val, err := resp.Session.State().Get(req.Key)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(val)
	return string(b), nil
}

func (m *defaultSwarm) writeState(ctx tool.Context, req struct {
	Key   string
	Value any
}) (string, error) {
	resp, err := m.sessionSvc.Get(context.Background(), &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID})
	if err != nil {
		return "", err
	}
	err = resp.Session.State().Set(req.Key, req.Value)
	if err != nil {
		return "", err
	}
	// We must also manually trigger a save since we are bypassing the Runner's auto-delta commit
	ev := session.NewEvent("")
	ev.Author = "System"
	ev.LLMResponse = model.LLMResponse{
		Content: genai.NewContentFromText(fmt.Sprintf("State updated: %s", req.Key), genai.Role("system")),
	}
	ev.Actions = session.EventActions{StateDelta: map[string]any{req.Key: req.Value}}

	err = m.sessionSvc.AppendEvent(context.Background(), resp.Session, ev)
	return "State updated successfully.", nil
}

func (m *defaultSwarm) spawnSubtask(ctx tool.Context, req struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Agent        string   `json:"agent"`
	Prompt       string   `json:"prompt"`
	Dependencies []string `json:"dependencies"`
	ParentID     string   `json:"parent_id,omitempty"`
}) (string, error) {
	if m.activeEngine == nil {
		return "", fmt.Errorf("no active execution engine")
	}
	
	// Prefix the new ID with the ParentID to ensure uniqueness if needed,
	// or let the agent handle it. Let's just use what they provide.
	span := Span{
		ID:           req.ID,
		Name:         req.Name,
		Agent:        req.Agent,
		Prompt:       req.Prompt,
		Dependencies: req.Dependencies,
		ParentID:     req.ParentID,
		Kind:         SpanKindAgent,
		Status:       SpanStatusPending,
	}
	m.activeEngine.AddSpans(span)
	
	if m.outChan != nil {
		m.outChan <- ObservableEvent{
			Timestamp: time.Now(),
			AgentName: req.Agent,
			SpanID:    req.ID,
			TaskName:  req.Name,
			ParentID:  req.ParentID,
			State:     AgentStatePending,
		}
	}
	
	return fmt.Sprintf("Subtask '%s' (%s) successfully spawned and added to the execution graph.", req.Name, req.ID), nil
}

func (m *defaultSwarm) IsDebug() bool         { return m.debugMode }
func (m *defaultSwarm) SetDebug(enabled bool) { m.debugMode = enabled }

func (m *defaultSwarm) Explain(ctx context.Context, traj Trajectory) (string, error) {
	trajJSON, _ := json.MarshalIndent(traj, "", "  ")
	prompt := fmt.Sprintf("You are the Swarm Historian. Explain WHY this path was taken. Concisely. Trajectory: %s", string(trajJSON))
	respIter := m.proModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}}, false)
	var exp string
	for resp, err := range respIter {
		if err != nil {
			return "", err
		}
		if resp.Content != nil && len(resp.Content.Parts) > 0 {
			exp += resp.Content.Parts[0].Text
		}
	}
	return strings.TrimSpace(exp), nil
}

func (m *defaultSwarm) Reload() error {
	var subAgents []agent.Agent
	var loadedSkills []*Skill
	var skill *Skill
	var instruction string

	// Find the skills directory by searching upwards
	absPath, _ := filepath.Abs(".")
	skillsPath := ""
	for {
		testPath := filepath.Join(absPath, "skills")
		if info, err := os.Stat(testPath); err == nil && info.IsDir() {
			skillsPath = testPath
			break
		}
		parentDir := filepath.Dir(absPath)
		if parentDir == absPath {
			break
		}
		absPath = parentDir
	}

	if skillsPath == "" {
		return fmt.Errorf("could not locate skills directory")
	}

	skillDirs := []string{}
	entries, err := os.ReadDir(skillsPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			skillDirs = append(skillDirs, filepath.Join(skillsPath, entry.Name()))
		}
	}

	m.agents = make(map[string]agent.Agent)
	var subAgentNames []string
	var subAgentDescriptions []string

	// 1. Load all skills into agents
	for _, dir := range skillDirs {
		var err error
		skill, err = LoadSkill(dir)
		if err != nil {
			continue
		}
		loadedSkills = append(loadedSkills, skill)
		// ... existing tool loading logic ...
		var skillTools []tool.Tool
		skillTools = append(skillTools, m.toolRegistry["request_replan"])
		for _, toolName := range skill.Manifest.Tools {
			if t, ok := m.toolRegistry[toolName]; ok {
				skillTools = append(skillTools, t)
			}
		}
		targetModel := m.proModel
		if skill.Manifest.Model == "flash" {
			targetModel = m.fastModel
		}

		// Core agents might need to skip the sub-agent suffix
		instruction = skill.Instructions
		if skill.Manifest.Name != "input_agent" && skill.Manifest.Name != "output_agent" && skill.Manifest.Name != "swarm_agent" && skill.Manifest.Name != "planning_agent" {
			instruction += "\n\nSUB-AGENT MODE: You are being invoked by the Swarm Agent to perform a specific span. Skip all greetings and introductory talk. Focus ONLY on executing the span and providing the results."
			subAgentNames = append(subAgentNames, skill.Manifest.Name)
			subAgentDescriptions = append(subAgentDescriptions, fmt.Sprintf("- **%s**: %s", skill.Manifest.Name, skill.Manifest.Description))
		}

		skillAgent, _ := llmagent.New(llmagent.Config{
			Name:        skill.Manifest.Name,
			Model:       targetModel,
			Description: skill.Manifest.Description,
			Instruction: instruction,
			Tools:       skillTools,
		})
		m.agents[skill.Manifest.Name] = skillAgent
		if skill.Manifest.Name != "input_agent" && skill.Manifest.Name != "output_agent" && skill.Manifest.Name != "swarm_agent" && skill.Manifest.Name != "planning_agent" {
			subAgents = append(subAgents, skillAgent)
		}
	}

	specialistsList := strings.Join(subAgentDescriptions, "\n")

	// 2. Specialized Setup for Swarm Agent (assign sub-agents)
	swarmAgent := m.agents["swarm_agent"]
	if swarmAgent == nil {
		return fmt.Errorf("swarm_agent skill not found")
	}

	// Re-initialize Swarm Agent with sub-agents and dynamically injected specialist names and descriptions
	skill, _ = LoadSkill(filepath.Join(skillsPath, "swarm-agent"))
	instruction = fmt.Sprintf(skill.Instructions, specialistsList)

	var swarmTools []tool.Tool
	swarmTools = append(swarmTools, m.toolRegistry["list_local_files"], m.toolRegistry["read_local_file"], m.toolRegistry["grep_search"])
	swarmTools = append(swarmTools, m.toolRegistry["read_state"], m.toolRegistry["write_state"], m.toolRegistry["spawn_subtask"])

	swarmAgent, _ = llmagent.New(llmagent.Config{
		Name:        "swarm_agent",
		Model:       m.fastModel,
		Instruction: instruction,
		Tools:       swarmTools,
		SubAgents:   subAgents,
	})
	m.agents["swarm_agent"] = swarmAgent

	// 3. Assign core instructions
	if _, ok := m.agents["input_agent"]; ok {
		sk, _ := LoadSkill(filepath.Join(skillsPath, "input-agent"))
		m.inputInstruction = sk.Instructions
	}
	if _, ok := m.agents["output_agent"]; ok {
		sk, _ := LoadSkill(filepath.Join(skillsPath, "output-agent"))
		m.outputInstruction = sk.Instructions
	}
	if _, ok := m.agents["planning_agent"]; ok {
		sk, _ := LoadSkill(filepath.Join(skillsPath, "planning-agent"))
		m.planningInstruction = sk.Instructions
	}

	m.skills = loadedSkills
	m.subAgentNames = subAgentNames
	m.run, _ = runner.New(runner.Config{AppName: "swarm-cli", Agent: swarmAgent, SessionService: m.sessionSvc})

	return nil
}

func (m *defaultSwarm) Skills() []*Skill { return m.skills }
func (m *defaultSwarm) AddContext(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m.pinnedContext[path] = string(b)
	return nil
}
func (m *defaultSwarm) DropContext(path string) {
	if path == "all" {
		m.pinnedContext = make(map[string]string)
	} else {
		delete(m.pinnedContext, path)
	}
}
func (m *defaultSwarm) ListContext() []string {
	var p []string
	for path := range m.pinnedContext {
		p = append(p, path)
	}
	return p
}
func (m *defaultSwarm) Reset() { m.sessionID = fmt.Sprintf("session_%d", rand.Int63()) }
func (m *defaultSwarm) ListModels(ctx context.Context) ([]ModelInfo, error) {
	client, err := genai.NewClient(ctx, m.clientCfg)
	if err != nil {
		return nil, err
	}
	var models []ModelInfo
	iter := client.Models.All(ctx)
	for mo, err := range iter {
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(mo.Name, "models/")
		models = append(models, ModelInfo{Name: name, DisplayName: mo.DisplayName, Description: mo.Description, Version: mo.Version})
	}
	return models, nil
}

func (m *defaultSwarm) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	resp, err := m.sessionSvc.List(ctx, &session.ListRequest{AppName: "swarm-cli", UserID: m.userID})
	if err != nil {
		return nil, err
	}
	var infos []SessionInfo
	for _, s := range resp.Sessions {
		summary := s.ID()
		var event struct{ Content string }
		err := m.db.Table("events").Select("content").Where("session_id = ? AND author = ?", s.ID(), "user").Order("timestamp DESC").Limit(1).Find(&event).Error
		if err == nil && event.Content != "" {
			summary = event.Content
		}
		if len(summary) > 40 {
			summary = summary[:37] + "..."
		}
		infos = append(infos, SessionInfo{ID: s.ID(), UpdatedAt: s.LastUpdateTime().Format("2006-01-02 15:04:05"), Summary: summary})
	}
	return infos, nil
}

func (m *defaultSwarm) Plan(ctx context.Context, prompt string) (*ExecutionGraph, error) {
	// Recompile the descriptions since they aren't stored on the struct
	var descriptions []string
	for _, name := range m.subAgentNames {
		if a, ok := m.agents[name]; ok {
			descriptions = append(descriptions, fmt.Sprintf("- **%s**: %s", name, a.Description()))
		}
	}
	specialistsList := strings.Join(descriptions, "\n")

	systemPrompt := fmt.Sprintf(m.planningInstruction, specialistsList)
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
		Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system"))},
	}, false)
	var jsonStr string
	for resp, err := range respIter {
		if err != nil {
			return nil, err
		}
		if resp.Content != nil && len(resp.Content.Parts) > 0 {
			jsonStr += resp.Content.Parts[0].Text
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "DEEP_PLAN_REQUIRED" {
		respIter = m.proModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}, Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system"))}}, false)
		jsonStr = ""
		for resp, err := range respIter {
			if err != nil {
				return nil, err
			}
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				jsonStr += resp.Content.Parts[0].Text
			}
		}
		jsonStr = strings.TrimSpace(jsonStr)
	}
	re := regexp.MustCompile("(?s)\\{.*\\}")
	match := re.FindString(jsonStr)
	if match != "" {
		jsonStr = match
	}
	var graph ExecutionGraph
	if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil {
		return nil, err
	}
	return &graph, nil
}

func (m *defaultSwarm) Chat(ctx context.Context, prompt string) (<-chan ObservableEvent, error) {
	out := make(chan ObservableEvent, 1000)
	// Record the user's initial prompt in the persistent session
	m.appendEvent(ctx, "user", prompt)

	go func() {
		defer close(out)
		swarmStartTime := time.Now()
		o := NewEngine(nil)

		defer func() {
			if o != nil {
				traj := o.GetTrajectory()
				traj.TotalDuration = time.Since(swarmStartTime).String()
				m.saveTrajectory(traj)

				if m.debugMode {
					b, _ := json.MarshalIndent(traj, "", "  ")
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateComplete, FinalContent: string(b)}
				}
			}
		}()

		// 1. Input Classification (Fast Path / CIA)
		inputStart := time.Now()
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateThinking, Thought: "Classifying intent…"}

		// Dynamically inject the last active agent to help the CIA detect digressions
		lastAgentName := "None (Starting new conversation)"
		if m.lastAgent != "" {
			lastAgentName = m.lastAgent
		}

		dynamicInstruction := m.inputInstruction + fmt.Sprintf("\n\nCURRENT CONTEXT: The last agent to respond was: %s.", lastAgentName)

		inputIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
			Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
			Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(dynamicInstruction, genai.Role("system"))},
		}, false)

		var inputResult string
		for resp, err := range inputIter {
			if err != nil {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateError, Error: fmt.Errorf("Input classification failed: %w", err)}
				return
			}
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				inputResult = resp.Content.Parts[0].Text
			}
			break
		}

		o.AddSpans(Span{
			ID: "input", Name: "Classification", Agent: "input_agent", Status: SpanStatusComplete,
			Kind:      SpanKindPlanner,
			StartTime: inputStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
			Duration: time.Since(inputStart).String(), Attributes: map[string]any{"gen_ai.prompt": prompt, "gen_ai.completion": inputResult},
		})

		var target string
		if strings.HasPrefix(inputResult, "ROUTE TO: swarm_agent") {
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateExecuting, Thought: "Rerouting to Swarm Agent…"}
			target = "swarm_agent"
		} else {
			target = "swarm_agent"
			if m.lastAgent != "" {
				target = m.lastAgent
			}
		}
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateComplete}

		var graph *ExecutionGraph
		var err error

		// 2. Swarm Coordination / Planning
		// If we are starting fresh or rerouting to Swarm Agent, we let it plan.
		if target == "swarm_agent" {
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm Agent", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateThinking, Thought: "Analyzing request…"}
			planStart := time.Now()
			graph, err = m.Plan(ctx, prompt)
			if err != nil {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm Agent", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateError, Error: fmt.Errorf("Coordination failed: %w", err)}
				return
			}

			if graph != nil && graph.ImmediateResponse == "" {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm Agent", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateComplete}
			}

			planJSON, _ := json.Marshal(graph)
			o.AddSpans(Span{
				ID: "coordination", Name: "Swarm Planning", Agent: "swarm_agent", Status: SpanStatusComplete,
				Kind:      SpanKindPlanner,
				StartTime: planStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
				Duration: time.Since(planStart).String(), Attributes: map[string]any{"gen_ai.prompt": prompt, "gen_ai.completion": string(planJSON)},
			})
		} else {
			// Direct execution with specialized agent (Node Autonomy)
			graph = &ExecutionGraph{Spans: []Span{{ID: "t1", Name: "Fulfill", Agent: target, Prompt: prompt}}}
		}

		// 3. Execution
		if len(graph.Spans) > 0 {
			events, _, err := m.Execute(ctx, graph, o)
			if err != nil {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "execution", TaskName: "Graph Execution", State: AgentStateError, Error: fmt.Errorf("Execution failed: %w", err)}
				return
			}
			for event := range events {
				out <- event
			}
			
			// If there was an immediate response bundled with the spans, we might want to still show it,
			// or assume the final span handles the output. Let's rely on the execution graph to provide the final output.
		} else if graph.ImmediateResponse != "" {
			if m.runOutputAgent(ctx, out, o, "Swarm Agent", graph.ImmediateResponse) {
				// Record the immediate response in the persistent session
				m.appendEvent(ctx, "model", graph.ImmediateResponse)
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm Agent", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateComplete, FinalContent: graph.ImmediateResponse}
			}
		}

		// 4. End of Chat
	}()
	return out, nil
}

func (m *defaultSwarm) Execute(ctx context.Context, g *ExecutionGraph, o *Engine) (<-chan ObservableEvent, *Engine, error) {
	if o == nil {
		o = NewEngine(g)
	}
	m.activeEngine = o
	out := make(chan ObservableEvent, 1000)
	m.outChan = out
	if g != nil {
		o.AddSpans(g.Spans...)
		// Broadcast initial topology so UI can build the tree immediately
		for _, s := range g.Spans {
			out <- ObservableEvent{
				Timestamp: time.Now(),
				AgentName: s.Agent,
				SpanID:    s.ID,
				TaskName:  s.Name,
				ParentID:  s.ParentID,
				State:     AgentStatePending,
			}
		}
	}

	go func() {
		defer close(out)
		defer func() {
			if m.activeEngine == o {
				m.activeEngine = nil
				m.outChan = nil
			}
		}()
		type completedSpan struct {
			ID     string
			Result string
			Status SpanStatus
			Replan bool
		}
		completed := make(chan completedSpan, 100)
		activeSpans := 0
		var wg sync.WaitGroup
		replanCount := 0
		const maxReplans = 3

	Loop:
		for {
			ready := o.GetReadySpans()
			for _, t := range ready {
				o.MarkActive(t.ID)
				activeSpans++
				wg.Add(1)
				go func(span Span) {
					defer wg.Done()
					result, status, needsReplan := m.executeSpan(ctx, out, o, span)
					completed <- completedSpan{ID: span.ID, Result: result, Status: status, Replan: needsReplan}
				}(t)
			}

			if activeSpans == 0 {
				if o.IsComplete() {
					break Loop
				} else {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateError, Error: fmt.Errorf("Deadlock detected in execution graph")}
					break Loop
				}
			}

			select {
			case <-ctx.Done():
				break Loop
			case done := <-completed:
				if done.Status == SpanStatusComplete {
					o.MarkComplete(done.ID, done.Result)
				} else {
					o.MarkFailed(done.ID)
				}
				activeSpans--
				// Handle dynamic replanning or subgraph expansion
				if done.Replan {
					if replanCount >= maxReplans {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateError, Error: fmt.Errorf("Maximum replan attempts reached. Halting loop")}
						continue
					}
					replanCount++
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateThinking, Thought: fmt.Sprintf("Replanning effort %d/%d…", replanCount, maxReplans)}
					newG, err := m.Plan(ctx, "Pivot: "+done.Result)
					if err == nil {
						o.AddSpans(newG.Spans...)
						// Broadcast replanned topology so UI can build the tree immediately
						for _, s := range newG.Spans {
							out <- ObservableEvent{
								Timestamp: time.Now(),
								AgentName: s.Agent,
								SpanID:    s.ID,
								TaskName:  s.Name,
								ParentID:  s.ParentID,
								State:     AgentStatePending,
							}
						}
					}
				}
			}
		}

		// Wait for all remaining active spans to recognize the context cancellation and gracefully exit
		wg.Wait()
	}()

	return out, o, nil
}

func (m *defaultSwarm) executeSpan(ctx context.Context, out chan<- ObservableEvent, o *Engine, span Span) (string, SpanStatus, bool) {
	targetAgent, ok := m.agents[span.Agent]
	if !ok {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("Agent not found: %s", span.Agent)}
		return "Agent not found", SpanStatusFailed, false
	}

	// Use a unique session ID for this specific span to prevent turn-order corruption
	// in the shared session service, especially during parallel execution.
	spanSessionID := fmt.Sprintf("%s/%s", m.sessionID, span.ID)

	// Ensure the span session exists in the database
	_, _ = m.sessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   "swarm-cli",
		UserID:    m.userID,
		SessionID: spanSessionID,
	})

	spanRunner, _ := runner.New(runner.Config{
		AppName:        "swarm-cli",
		Agent:          targetAgent,
		SessionService: m.sessionSvc,
	})

	out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateSpawning, Thought: "Starting…"}

	// Telemetry and Observation
	telemetryChan := make(chan string, 100)
	// Bind the telemetry channel context to the spanCtx so it cancels immediately when the runner returns
	spanCtx, cancel := context.WithCancel(context.WithValue(ctx, telemetryContextKey{}, (chan<- string)(telemetryChan)))
	defer cancel()

	telemetryDone := make(chan struct{})
	var obsWg sync.WaitGroup
	obsWg.Add(1)
	go func() {
		defer close(telemetryDone)
		defer obsWg.Done()
		var history []string
		var newLines bool

		baseInterval := 2 * time.Second
		interval := baseInterval
		maxInterval := 60 * time.Second
		timer := time.NewTimer(interval)
		defer timer.Stop()

		for {
			select {
			case <-spanCtx.Done(): // Fix: Use spanCtx instead of ctx to immediately stop the observer when the agent finishes
				return
			case line, ok := <-telemetryChan:
				if !ok {
					return
				}
				history = append(history, line)
				if len(history) > 100 {
					history = history[len(history)-100:]
				}
				newLines = true
			case <-timer.C:
				if newLines {
					newLines = false

					histCopy := make([]string, len(history))
					copy(histCopy, history)
					obsWg.Add(1)
					go func(hist []string) {
						defer obsWg.Done()
						obsCtx, obsCancel := context.WithTimeout(spanCtx, 10*time.Second) // Fix: Tie observer timeout directly to spanCtx
						defer obsCancel()

						obsPrompt := fmt.Sprintf("Monitor: %s. Recent Telemetry:\n%s\n\nTask: Output a concise 3-8 word phrase summarizing the current activity (e.g., 'Searching for authentication logic...' or 'Running unit tests...'). If you detect an infinite loop or severe error, output 'INTERVENE: [reason]'.", targetAgent.Name(), strings.Join(hist, "\n"))

						respIter := m.fastModel.GenerateContent(obsCtx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(obsPrompt, genai.Role("user"))}}, false)
						for resp, err := range respIter {
							if err != nil {
								// Silently ignore transient errors in the background observer loop
								break
							}
							if resp.Content != nil && len(resp.Content.Parts) > 0 {
								text := strings.TrimSpace(resp.Content.Parts[0].Text)
								if strings.HasPrefix(text, "INTERVENE:") {
									out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Observer", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateWaiting, ObserverSummary: text}
								} else {
									text = strings.TrimSuffix(text, ".")
									out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateExecuting, ObserverSummary: text}
								}
							}
							break
						}
					}(histCopy)

					interval = interval * 2
					if interval > maxInterval {
						interval = maxInterval
					}
				} else {
					// No new lines this cycle. Reset backoff so next burst is summarized quickly.
					interval = baseInterval
				}
				timer.Reset(interval)
			}
		}
	}()

	// Fetch and inject structured Session State (Node Autonomy coordination)
	var stateParts []string
	if sessResp, err := m.sessionSvc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID}); err == nil {
		sessionState := sessResp.Session.State()
		stateMap := make(map[string]any)
		for k, v := range sessionState.All() {
			stateMap[k] = v
		}
		if len(stateMap) > 0 {
			stateJSON, _ := json.MarshalIndent(stateMap, "", "  ")
			stateParts = append(stateParts, fmt.Sprintf("CURRENT SESSION STATE (Structured Data):\n%s", string(stateJSON)))
		}
	}

	// Inject results from dependencies into the prompt (Node Autonomy)
	contextMap := o.GetContext()
	var contextParts []string
	for _, depID := range span.Dependencies {
		if res, ok := contextMap[depID]; ok {
			if len(res) > 8000 {
				res = res[:8000] + "\n\n...[Content truncated to preserve context limits]..."
			}
			contextParts = append(contextParts, fmt.Sprintf("Output from previous span (%s):\n%s", depID, res))
		}
	}

	// Fetch and inject global conversation history
	var historyParts []string
	if sessResp, err := m.sessionSvc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID}); err == nil {
		for ev := range sessResp.Session.Events().All() {
			if ev.Content != nil {
				for _, part := range ev.Content.Parts {
					if part.Text != "" {
						text := part.Text
						if len(text) > 4000 {
							text = text[:4000] + "\n\n...[Content truncated to preserve context limits]..."
						}
						historyParts = append(historyParts, fmt.Sprintf("[%s]: %s", ev.Author, text))
					}
				}
			}
		}
	}

	promptStr := fmt.Sprintf("TASK: %s\nINSTRUCTIONS: %s", span.Name, span.Prompt)
	promptStr += fmt.Sprintf("\n\n### TASK CONTEXT\nYour current Task ID is: %s\nIf you need to spawn subtasks, use this ID as their 'parent_id' or explicitly in their 'dependencies' list to ensure they block this task's completion if necessary.", span.ID)

	if len(stateParts) > 0 {
		promptStr = promptStr + "\n\n### SESSION STATE\n" + strings.Join(stateParts, "\n")
	}
	if len(historyParts) > 0 {
		promptStr = promptStr + "\n\n### CONVERSATION HISTORY (FOR CONTEXT)\n" + strings.Join(historyParts, "\n")
	}
	if len(contextParts) > 0 {
		promptStr = promptStr + "\n\n### RESULTS FROM PREVIOUS TASKS\n" + strings.Join(contextParts, "\n\n")
	}

	// Execute with the unique spanSessionID
	events := spanRunner.Run(spanCtx, m.userID, spanSessionID, genai.NewContentFromText(promptStr, genai.Role("user")), agent.RunConfig{})

	var full strings.Builder
	var needsReplan bool
	activeToolSpans := make(map[string][]Span)

	for event, err := range events {
		if err != nil {
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("Error encountered: %w", err)}
			full.WriteString("\n\nERROR: " + err.Error())
			needsReplan = true
			break
		}
		var thoughtBroadcasted bool
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.FunctionCall != nil {
					if part.FunctionCall.Name == "request_replan" {
						needsReplan = true
					}
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateExecuting, ToolName: part.FunctionCall.Name, ToolArgs: part.FunctionCall.Args}

					// Record tool span start
					toolStart := time.Now()
					toolID := fmt.Sprintf("%s-tool-%s-%d", span.ID, part.FunctionCall.Name, toolStart.UnixNano())
					toolTask := Span{
						ID: toolID, ParentID: span.ID, TraceID: span.TraceID,
						Name: part.FunctionCall.Name, Kind: SpanKindTool, Status: SpanStatusActive,
						StartTime:  toolStart.Format(time.RFC3339Nano),
						Attributes: map[string]any{"gen_ai.tool_args": part.FunctionCall.Args},
					}
					o.AddSpans(toolTask)
					activeToolSpans[part.FunctionCall.Name] = append(activeToolSpans[part.FunctionCall.Name], toolTask)
				}
				if part.FunctionResponse != nil {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, ToolName: part.FunctionResponse.Name}

					// Record tool span completion (pop from queue)
					if spans, ok := activeToolSpans[part.FunctionResponse.Name]; ok && len(spans) > 0 {
						t := spans[0]
						activeToolSpans[part.FunctionResponse.Name] = spans[1:]

						now := time.Now()
						t.Status = SpanStatusComplete
						t.EndTime = now.Format(time.RFC3339Nano)
						if t.StartTime != "" {
							start, _ := time.Parse(time.RFC3339Nano, t.StartTime)
							t.Duration = now.Sub(start).String()
						}
						resJSON, _ := json.Marshal(part.FunctionResponse.Response)
						t.Attributes["gen_ai.tool_result"] = string(resJSON)
						o.AddSpans(t) // Update the span in the engine
					}
				}
				if part.Thought {
					if !thoughtBroadcasted {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, Thought: "Thinking deeply..."}
						thoughtBroadcasted = true
					}
				} else if part.Text != "" {
					full.WriteString(part.Text)
				}
			}
		}
	}

	finalText := full.String()
	close(telemetryChan)
	<-telemetryDone
	obsWg.Wait()

	// Ensure all tool spans are closed (Node Autonomy cleanup)
	for _, spans := range activeToolSpans {
		for _, t := range spans {
			now := time.Now()
			t.Status = SpanStatusFailed
			t.EndTime = now.Format(time.RFC3339Nano)
			t.Attributes["gen_ai.tool_result"] = "Tool execution interrupted or incomplete"
			o.AddSpans(t)
		}
	}

	if finalText == "" {
		return "No response emitted by agent.", SpanStatusFailed, false
	}

	// Update the global state of the last agent to respond
	m.lastAgent = targetAgent.Name()

	// If there was a hard error, skip Output Agent and immediately return with failed status and replan flag
	if strings.Contains(finalText, "\n\nERROR: ") && needsReplan {
		m.appendEvent(ctx, "model", fmt.Sprintf("[%s]: %s", targetAgent.Name(), finalText))
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, FinalContent: finalText}
		return finalText, SpanStatusFailed, true
	}

	// Output Sanity Check (Output Agent)
	if m.runOutputAgent(ctx, out, o, targetAgent.Name(), finalText) {
		// Record the response in the main conversation history
		m.appendEvent(ctx, "model", fmt.Sprintf("[%s]: %s", targetAgent.Name(), finalText))
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateComplete, FinalContent: finalText}
		return finalText, SpanStatusComplete, needsReplan
	}

	out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, Thought: "Output Agent rejected the response as problematic."}
	return "Output Agent rejected response.", SpanStatusComplete, true
}

func (m *defaultSwarm) appendEvent(ctx context.Context, author, content string) {
	if m.sessionSvc == nil {
		return
	}
	// Fetch the actual session object to fulfill the interface
	resp, err := m.sessionSvc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID})
	if err != nil {
		return
	}
	_ = m.sessionSvc.AppendEvent(ctx, resp.Session, &session.Event{
		Timestamp: time.Now(),
		Author:    author,
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText(content, genai.Role(author)),
		},
	})
}

func (m *defaultSwarm) runOutputAgent(ctx context.Context, out chan<- ObservableEvent, o *Engine, agentName, content string) bool {
	coaStart := time.Now()
	coaPrompt := fmt.Sprintf("Sanity check worker: %s. Response: %s. RULE: Output ONLY 'OK' or 'FIX: [reason]'.", agentName, content)
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(coaPrompt, genai.Role("user"))},
		Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(m.outputInstruction, genai.Role("system"))},
	}, false)

	approved := true
	var res string
	for resp, err := range respIter {
		if err == nil && resp.Content != nil && len(resp.Content.Parts) > 0 {
			res = resp.Content.Parts[0].Text
			if strings.HasPrefix(res, "FIX:") {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Output Agent", State: AgentStateError, Thought: "Rejected: " + res}
				approved = false
			}
		}
		break
	}

	// Update UI card state
	if approved {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Output Agent", State: AgentStateComplete, FinalContent: "OK"}
	}

	if o != nil {
		o.AddSpans(Span{
			ID: "output-agent-" + fmt.Sprintf("%d", coaStart.UnixNano()), Name: "Sanity Check", Agent: "output_agent",
			Status: SpanStatusComplete, StartTime: coaStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
			Duration: time.Since(coaStart).String(), Attributes: map[string]any{"gen_ai.prompt": coaPrompt, "gen_ai.completion": res},
		})
	}
	return approved
}

func (m *defaultSwarm) Rewind(n int) error {
	if n <= 0 {
		return nil
	}
	var evs []struct{ Timestamp string }
	err := m.db.Table("events").Select("timestamp").Where("session_id = ? AND author = ?", m.sessionID, "user").Order("timestamp DESC").Limit(n).Find(&evs).Error
	if err != nil {
		return err
	}
	if len(evs) < n {
		return m.db.Table("events").Where("session_id = ?", m.sessionID).Delete(nil).Error
	}
	return m.db.Table("events").Where("session_id = ? AND timestamp >= ?", m.sessionID, evs[len(evs)-1].Timestamp).Delete(nil).Error
}

func (m *defaultSwarm) saveTrajectory(traj Trajectory) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".config", "swarm", "trajectories")
	_ = os.MkdirAll(dir, 0755)

	filename := fmt.Sprintf("%s.json", traj.TraceID)
	if m.sessionID != "" {
		filename = fmt.Sprintf("%s_%s.json", m.sessionID, traj.TraceID)
	}
	// Sanitize output path
	filename = strings.ReplaceAll(filename, "/", "_")
	path := filepath.Join(dir, filename)

	b, _ := json.MarshalIndent(traj, "", "  ")
	_ = os.WriteFile(path, b, 0644)
}

// timeoutModel wraps a model.LLM to enforce a strict per-request execution timeout
type timeoutModel struct {
	underlying model.LLM
	timeout    time.Duration
}

func (t *timeoutModel) Name() string {
	return t.underlying.Name()
}

func (t *timeoutModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		timeoutCtx, cancel := context.WithTimeout(ctx, t.timeout)
		defer cancel()

		for resp, err := range t.underlying.GenerateContent(timeoutCtx, req, stream) {
			if !yield(resp, err) {
				return
			}
		}
	}
}
