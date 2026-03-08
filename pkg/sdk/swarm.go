package sdk

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"iter"
	"math/rand"
	"net"
	"net/http"
	"os"
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
	PGID     int

	// Output
	Thought      string
	FinalContent string
	Error        error

	// Observers
	ObserverSummary string
}

type Reflection struct {
	IsResolved     bool     `json:"is_resolved"`
	NeedsUserInput bool     `json:"needs_user_input"`
	Reasoning      string   `json:"reasoning"`
	NextSteps      string   `json:"next_steps"`
	NewFacts       []string `json:"new_facts,omitempty"`
}

type Swarm interface {
	AddContext(path string) error
	DropContext(path string)
	ListContext() []string
	ListFacts(limit int) ([]string, error)
	Plan(ctx context.Context, prompt string, traj Trajectory) (*ExecutionGraph, error)
	Reflect(ctx context.Context, prompt string, traj Trajectory) (*Reflection, error)
	Execute(ctx context.Context, g *ExecutionGraph, o *Engine) (<-chan ObservableEvent, *Engine, error)
	Chat(ctx context.Context, prompt string) (<-chan ObservableEvent, error)
	SummarizeState(ctx context.Context, state string) (string, error)
	Reset()
	Reload() error
	Rewind(n int) error
	Skills() []*Skill
	Memory() HierarchicalMemory
	ListModels(ctx context.Context) ([]ModelInfo, error)
	ListSessions(ctx context.Context) ([]SessionInfo, error)
	SessionID() string
	SetDebug(enabled bool)
	IsDebug() bool
	Explain(ctx context.Context, traj Trajectory) (string, error)
	Close() error
}

type telemetryContextKey struct{}

type defaultSwarm struct {
	mu                  sync.RWMutex
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
	routingInstruction  string
	planningInstruction string
	debugMode           bool
	flashModel          model.LLM
	proModel            model.LLM
	fastModel           model.LLM
	toolRegistry        map[string]tool.Tool
	subAgentNames       []string
	agents              map[string]agent.Agent
	lastAgent           string  // Tracks the last agent to respond
	activeEngine        *Engine // The currently executing Engine, used for dynamic task mutability
	outChan             chan<- ObservableEvent
	wg                  sync.WaitGroup
	trajectoryDir       string
	forceDonate         bool
	telemetryConfigured bool
	memory              HierarchicalMemory
}

type SwarmConfig struct {
	Model             model.LLM
	ResumeLastSession bool
	Debug             bool
	DatabaseURI       string
	TrajectoryDir     string // Optional override for where trajectories are saved (e.g., "eval_trajectories")
	ForceDonate       bool   // If true, forces the "donate" telemetry flag to true for this run
}

func NewSwarm(cfg ...SwarmConfig) (Swarm, error) {
	ctx := context.Background()
	debugMode := false
	if len(cfg) > 0 && cfg[0].Debug {
		debugMode = true
	}
	trajectoryDir := "trajectories"
	forceDonate := false
	if len(cfg) > 0 {
		if cfg[0].TrajectoryDir != "" {
			trajectoryDir = cfg[0].TrajectoryDir
		}
		forceDonate = cfg[0].ForceDonate
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
		// We disable HTTP/2 to prevent 'GOAWAY' and stream concurrency errors
		// during massive parallel execution (fan-outs).
		clientConfig.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     false,
				TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second, // Max 60s wait for first byte
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
			if strings.Contains(err.Error(), "api key is required") {
				return nil, fmt.Errorf("Missing API Key: Please set the GOOGLE_API_KEY environment variable. You can get one from https://aistudio.google.dev/app/apikey")
			}
			return nil, err
		}
		proModel, err = gemini.NewModel(ctx, proModelName, clientConfig)
		if err != nil {
			if strings.Contains(err.Error(), "api key is required") {
				return nil, fmt.Errorf("Missing API Key: Please set the GOOGLE_API_KEY environment variable. You can get one from https://aistudio.google.dev/app/apikey")
			}
			return nil, err
		}
		fastModel, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig)
		if err != nil {
			if strings.Contains(err.Error(), "api key is required") {
				return nil, fmt.Errorf("Missing API Key: Please set the GOOGLE_API_KEY environment variable. You can get one from https://aistudio.google.dev/app/apikey")
			}
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
	bashExecute, _ := functiontool.New(functiontool.Config{
		Name:        "bash_execute",
		Description: "Executes a shell command. Use this for running tests, git operations, or inspecting the system. If starting a server, watcher, or any process that does not exit immediately, you MUST set is_background: true to avoid hanging the system. The tool will return the Process Group ID (PGID) for background tasks, which you can later kill via `kill -- -PGID`.",
	}, bashExecuteTool)
	webFetchTool, _ := functiontool.New(functiontool.Config{Name: "web_fetch"}, webFetch)
	googleSearchTool, _ := functiontool.New(functiontool.Config{Name: "google_search"}, googleSearchFunc)
	replanTool, _ := functiontool.New(functiontool.Config{Name: "request_replan"}, requestReplan)

	var dbURI string
	if len(cfg) > 0 {
		dbURI = cfg[0].DatabaseURI
	}
	if dbURI == "" {
		dbDir, _ := GetConfigDir()
		dbURI = filepath.Join(dbDir, "sessions.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	}
	dialector := sqlite.Open(dbURI)
	gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}

	// Prevent SQLite 'database is locked' errors by restricting concurrent writes in the Go pool
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(1)
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
		sessionID = fmt.Sprintf("session_%d", rand.Int63()) //nolint:gosec // session IDs do not require cryptographic security
	}
	_, _ = sessionSvc.Create(ctx, &session.CreateRequest{AppName: "swarm-cli", UserID: "local_user", SessionID: sessionID})

	globalCfg, _ := LoadConfig()
	telemetryConfigured := false
	if globalCfg != nil {
		telemetryConfigured = globalCfg.Telemetry
	}

	root := FindProjectRoot()
	semanticMem, err := NewSemanticMemory(root)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize semantic memory: %w", err)
	}

	m := &defaultSwarm{
		db: db, sessionSvc: sessionSvc, userID: "local_user", sessionID: sessionID, clientCfg: clientConfig, pinnedContext: make(map[string]string), flashModel: flashModel, proModel: proModel, fastModel: fastModel, debugMode: debugMode,
		trajectoryDir:       trajectoryDir,
		forceDonate:         forceDonate,
		telemetryConfigured: telemetryConfigured,
	}
	m.memory = NewHierarchicalMemory(m, NewEpisodicMemory(sessionSvc, m.userID), semanticMem, m)

	readStateTool, _ := functiontool.New(functiontool.Config{Name: "read_state"}, m.readState)
	writeStateTool, _ := functiontool.New(functiontool.Config{Name: "write_state"}, m.writeState)
	retrieveFactTool, _ := functiontool.New(functiontool.Config{Name: "retrieve_fact"}, m.retrieveFact)
	spawnSubtaskTool, _ := functiontool.New(functiontool.Config{Name: "spawn_subtask"}, m.spawnSubtask)
	m.toolRegistry = map[string]tool.Tool{
		"list_local_files": listTool, "read_local_file": readTool, "grep_search": grepTool, "write_local_file": writeTool, "git_commit": gitCommit, "git_push": gitPush, "bash_execute": bashExecute, "web_fetch": webFetchTool, "google_search": googleSearchTool, "request_replan": replanTool,
		"read_state": readStateTool, "write_state": writeStateTool, "retrieve_fact": retrieveFactTool, "spawn_subtask": spawnSubtaskTool,
	}

	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *defaultSwarm) readState(ctx tool.Context, req struct{ Key string }) (string, error) {
	if m.memory == nil || m.memory.Episodic() == nil {
		return "", fmt.Errorf("episodic memory not initialized")
	}
	val, err := m.memory.Episodic().GetState(context.Background(), m.sessionID, req.Key)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(val)
	return string(b), nil
}

func (m *defaultSwarm) writeState(ctx tool.Context, req struct {
	Key   string
	Value any
},
) (string, error) {
	if m.memory == nil || m.memory.Episodic() == nil {
		return "", fmt.Errorf("episodic memory not initialized")
	}
	err := m.memory.Episodic().SetState(context.Background(), m.sessionID, req.Key, req.Value)
	if err != nil {
		return "", err
	}
	return "State updated successfully.", nil
}

func (m *defaultSwarm) retrieveFact(ctx tool.Context, req struct {
	Query string
	Limit int
},
) (string, error) {
	if m.memory == nil || m.memory.Semantic() == nil {
		return "", fmt.Errorf("semantic memory not initialized")
	}

	limit := req.Limit
	if limit == 0 {
		limit = 5
	}

	facts, err := m.memory.Semantic().Retrieve(req.Query, limit)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve facts: %w", err)
	}

	if len(facts) == 0 {
		return "No relevant facts found in semantic memory.", nil
	}

	b, _ := json.MarshalIndent(facts, "", "  ")
	return string(b), nil
}

func (m *defaultSwarm) spawnSubtask(ctx tool.Context, req struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Agent        string   `json:"agent"`
	Prompt       string   `json:"prompt"`
	Dependencies []string `json:"dependencies"`
	ParentID     string   `json:"parent_id,omitempty"`
},
) (string, error) {
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

func (m *defaultSwarm) Close() error {
	m.wg.Wait()
	return nil
}

func (m *defaultSwarm) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var subAgents []agent.Agent
	var loadedSkills []*Skill
	var skill *Skill
	var instruction string
	loadedContext, loadedFiles := LoadContextFiles()

	// Add implicitly loaded files to pinned context so they show up in ListContext()
	for _, f := range loadedFiles {
		m.pinnedContext[f] = "" // Mark as loaded but don't duplicate content in memory
	}

	// 1. Find the local project skills directory (search upwards)
	localSkillsPath := ""
	root := FindProjectRoot()
	testPath := filepath.Join(root, "skills")
	if info, err := os.Stat(testPath); err == nil && info.IsDir() {
		localSkillsPath = testPath
	}

	// 2. Find the global config skills directory
	globalSkillsPath := ""
	if configDir, err := GetConfigDir(); err == nil {
		globalSkillsPath = filepath.Join(configDir, "skills")
		_ = os.MkdirAll(globalSkillsPath, 0o755)
	}

	// Gather all skill directories
	skillDirs := []string{}

	// Collect local project skills
	if localSkillsPath != "" {
		if entries, err := os.ReadDir(localSkillsPath); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					skillDirs = append(skillDirs, filepath.Join(localSkillsPath, entry.Name()))
				}
			}
		}
	}

	// Collect global config skills
	if globalSkillsPath != "" {
		if entries, err := os.ReadDir(globalSkillsPath); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					skillDirs = append(skillDirs, filepath.Join(globalSkillsPath, entry.Name()))
				}
			}
		}
	}

	if len(skillDirs) == 0 {
		return fmt.Errorf("could not locate any skills directories")
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
			instruction += "\n\nSUB-AGENT MODE: You are being invoked by Swarm to perform a specific span. Skip all greetings and introductory talk. Focus ONLY on executing the span and providing the results."
			subAgentNames = append(subAgentNames, skill.Manifest.Name)
			subAgentDescriptions = append(subAgentDescriptions, fmt.Sprintf("- **%s**: %s", skill.Manifest.Name, skill.Manifest.Description))
		}

		skillAgent, _ := llmagent.New(llmagent.Config{
			Name:        skill.Manifest.Name,
			Model:       targetModel,
			Description: skill.Manifest.Description,
			Instruction: instruction + loadedContext,
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

	// Helper to find a specific skill by directory name across all paths
	findSkillPath := func(name string) string {
		for _, dir := range skillDirs {
			if filepath.Base(dir) == name {
				return dir
			}
		}
		return ""
	}

	// Re-initialize Swarm Agent with sub-agents and dynamically injected specialist names and descriptions
	if p := findSkillPath("swarm-agent"); p != "" {
		skill, _ = LoadSkill(p)
		if skill != nil {
			instruction = fmt.Sprintf(skill.Instructions, specialistsList)
		}
	}

	var swarmTools []tool.Tool
	swarmTools = append(swarmTools, m.toolRegistry["list_local_files"], m.toolRegistry["read_local_file"], m.toolRegistry["grep_search"])
	swarmTools = append(swarmTools, m.toolRegistry["read_state"], m.toolRegistry["write_state"], m.toolRegistry["spawn_subtask"])

	swarmAgent, _ = llmagent.New(llmagent.Config{
		Name:        "swarm_agent",
		Model:       m.fastModel,
		Instruction: instruction + loadedContext,
		Tools:       swarmTools,
		SubAgents:   subAgents,
	})
	m.agents["swarm_agent"] = swarmAgent

	// 3. Assign core instructions
	if _, ok := m.agents["input_agent"]; ok {
		if p := findSkillPath("input-agent"); p != "" {
			if sk, err := LoadSkill(p); err == nil {
				m.inputInstruction = sk.Instructions + loadedContext
			}
		}
	}
	if _, ok := m.agents["output_agent"]; ok {
		if p := findSkillPath("output-agent"); p != "" {
			if sk, err := LoadSkill(p); err == nil {
				m.outputInstruction = sk.Instructions + loadedContext
			}
		}
	}
	if _, ok := m.agents["routing_agent"]; ok {
		if p := findSkillPath("routing-agent"); p != "" {
			if sk, err := LoadSkill(p); err == nil {
				m.routingInstruction = sk.Instructions + loadedContext
			}
		}
	}
	if _, ok := m.agents["planning_agent"]; ok {
		if p := findSkillPath("planning-agent"); p != "" {
			if sk, err := LoadSkill(p); err == nil {
				m.planningInstruction = sk.Instructions + loadedContext
			}
		}
	}

	m.skills = loadedSkills
	m.subAgentNames = subAgentNames
	m.run, _ = runner.New(runner.Config{AppName: "swarm-cli", Agent: swarmAgent, SessionService: m.sessionSvc})

	return nil
}

func (m *defaultSwarm) AddSpans(spans ...Span) {
	if m.activeEngine != nil {
		m.activeEngine.AddSpans(spans...)
	}
}

func (m *defaultSwarm) GetTrajectory() Trajectory {
	if m.activeEngine != nil {
		return m.activeEngine.GetTrajectory()
	}
	return Trajectory{}
}

func (m *defaultSwarm) Load() (string, error) {
	return LoadMemory()
}

func (m *defaultSwarm) Save(fact string) error {
	return SaveMemory(fact)
}

func (m *defaultSwarm) GlobalStats() MemoryStats {
	content, _ := LoadMemory()

	tokens := len(content) / 4
	count := 1

	// Add pinned context files to global memory tokens
	for p, c := range m.pinnedContext {
		count++
		if c == "" {
			b, _ := os.ReadFile(p)
			tokens += len(b) / 4
		} else {
			tokens += len(c) / 4
		}
	}

	// Add skills and instructions size
	for _, sk := range m.skills {
		count++
		tokens += len(sk.Instructions) / 4
	}

	return MemoryStats{
		Name:          "Global Memory (Tier 4)",
		Count:         count,
		TokenEstimate: tokens,
	}
}

func (m *defaultSwarm) GetContext() map[string]string {
	if m.activeEngine != nil {
		return m.activeEngine.GetContext()
	}
	return make(map[string]string)
}

func (m *defaultSwarm) WorkingStats() MemoryStats {
	if m.activeEngine == nil {
		return MemoryStats{Name: "Working Memory (Tier 1)"}
	}
	traj := m.activeEngine.GetTrajectory()
	tokens := 0
	for _, s := range traj.Spans {
		tokens += len(s.Prompt) / 4
		if res, ok := s.Attributes["gen_ai.completion"].(string); ok {
			tokens += len(res) / 4
		}
	}
	return MemoryStats{
		Name:          "Working Memory (Tier 1)",
		Count:         len(traj.Spans),
		TokenEstimate: tokens,
	}
}

func (m *defaultSwarm) Skills() []*Skill { return m.skills }

func (m *defaultSwarm) Memory() HierarchicalMemory { return m.memory }

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

func (m *defaultSwarm) ListFacts(limit int) ([]string, error) {
	if m.memory == nil || m.memory.Semantic() == nil {
		return nil, fmt.Errorf("semantic memory not initialized")
	}
	return m.memory.Semantic().List(limit)
}

func (m *defaultSwarm) Reset() {
	m.sessionID = fmt.Sprintf("session_%d", rand.Int63()) //nolint:gosec // session IDs do not require cryptographic security
}
func (m *defaultSwarm) SessionID() string { return m.sessionID }
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
		if len(summary) > 80 {
			summary = summary[:77] + "..."
		}
		infos = append(infos, SessionInfo{ID: s.ID(), UpdatedAt: s.LastUpdateTime().Format("2006-01-02 15:04:05"), Summary: summary})
	}
	return infos, nil
}

// extractJSON attempts to locate and extract a JSON object or array from a string
// that may be wrapped in markdown or contain leading/trailing conversational text.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")

	start := -1
	end := -1

	if startObj != -1 && (startArr == -1 || startObj < startArr) {
		start = startObj
		end = strings.LastIndex(s, "}")
	} else if startArr != -1 && (startObj == -1 || startArr < startObj) {
		start = startArr
		end = strings.LastIndex(s, "]")
	}

	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}

	return s
}

func (m *defaultSwarm) Plan(ctx context.Context, prompt string, traj Trajectory) (*ExecutionGraph, error) {
	// Dynamically reload skills from disk so that skills created during this session (e.g. by the skill_builder_agent) are immediately available.
	_ = m.Reload()

	promptForLLM := prompt
	if len(traj.Spans) > 0 {
		b, _ := json.MarshalIndent(traj.Spans, "", "  ")
		promptForLLM = promptForLLM + "\n\n### PREVIOUS EXECUTION TRAJECTORY:\n" + string(b)
	}

	// Recompile the descriptions since they aren't stored on the struct
	m.mu.RLock()
	var descriptions []string
	for _, name := range m.subAgentNames {
		if a, ok := m.agents[name]; ok {
			descriptions = append(descriptions, fmt.Sprintf("- **%s**: %s", name, a.Description()))
		}
	}
	specialistsList := strings.Join(descriptions, "\n")
	routingInstruction := m.routingInstruction
	planningInstruction := m.planningInstruction
	m.mu.RUnlock()

	routingPrompt := fmt.Sprintf(routingInstruction, specialistsList)

	// Inject any critical semantic facts (like tool deprecations) or facts directly relevant to the user's prompt
	if m.memory != nil && m.memory.Semantic() != nil {
		var injectedFacts []string

		// 1. Always look for system-level tool failures
		if sysFacts, err := m.memory.Semantic().Retrieve("TOOL FAILURE OFFLINE", 3); err == nil && len(sysFacts) > 0 {
			injectedFacts = append(injectedFacts, sysFacts...)
		}

		// 2. Look for semantic memories related to the user's actual prompt (not the trajectory json)
		if userFacts, err := m.memory.Semantic().Retrieve(prompt, 3); err == nil && len(userFacts) > 0 {
			injectedFacts = append(injectedFacts, userFacts...)
		}

		if len(injectedFacts) > 0 {
			routingPrompt += "\n\n### CRITICAL SYSTEM FACTS (SEMANTIC MEMORY):\n" + strings.Join(injectedFacts, "\n")
		}
	}
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(promptForLLM, genai.Role("user"))},
		Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(routingPrompt, genai.Role("system"))},
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
	if strings.Contains(jsonStr, "DEEP_PLAN_REQUIRED") {
		planningPrompt := fmt.Sprintf(planningInstruction, specialistsList)

		if m.memory != nil && m.memory.Semantic() != nil {
			var injectedFacts []string
			if facts, err := m.memory.Semantic().Retrieve("TOOL FAILURE OFFLINE", 5); err == nil && len(facts) > 0 {
				injectedFacts = append(injectedFacts, facts...)
			}
			if userFacts, err := m.memory.Semantic().Retrieve(prompt, 5); err == nil && len(userFacts) > 0 {
				injectedFacts = append(injectedFacts, userFacts...)
			}
			if len(injectedFacts) > 0 {
				planningPrompt += "\n\n### RELEVANT SYSTEM FACTS (SEMANTIC MEMORY):\n" + strings.Join(injectedFacts, "\n")
			}
		}

		respIter = m.proModel.GenerateContent(ctx, &model.LLMRequest{
			Contents: []*genai.Content{genai.NewContentFromText(promptForLLM, genai.Role("user"))},
			Config: &genai.GenerateContentConfig{
				SystemInstruction: genai.NewContentFromText(planningPrompt, genai.Role("system")),
				ResponseMIMEType:  "application/json",
				ResponseSchema: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"immediate_response": {Type: genai.TypeString, Description: "Use this to short-circuit the planning phase and respond directly to trivial conversational requests without delegating to sub-agents."},
						"spans": {
							Type: genai.TypeArray,
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"id":           {Type: genai.TypeString, Description: "A unique identifier for this span, like 't1'."},
									"operation_name": {Type: genai.TypeString, Description: "A short, descriptive name for the task."},
									"agent":        {Type: genai.TypeString, Description: "The exact name of the sub-agent to execute this span."},
									"prompt":       {Type: genai.TypeString, Description: "The comprehensive instruction prompt for the sub-agent."},
									"dependencies": {
										Type:  genai.TypeArray,
										Items: &genai.Schema{Type: genai.TypeString},
										Description: "An array of span IDs that must successfully complete before this span can begin.",
									},
								},
								Required: []string{"id", "operation_name", "agent", "prompt", "dependencies"},
							},
						},
					},
				},
			},
		}, false)
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
	jsonStr = strings.TrimSpace(jsonStr)
	
	// Remove markdown ticks if the LLM ignores the schema constraints
	if strings.HasPrefix(jsonStr, "```json") {
		jsonStr = strings.TrimPrefix(jsonStr, "```json")
		jsonStr = strings.TrimSuffix(jsonStr, "```")
		jsonStr = strings.TrimSpace(jsonStr)
	}
	
	jsonStr = extractJSON(jsonStr)

	var graph ExecutionGraph

	// If extractJSON couldn't find a JSON object or array, and the string is just conversational text,
	// gracefully wrap it in an immediate response instead of crashing.
	if jsonStr != "" && !strings.HasPrefix(jsonStr, "{") && !strings.HasPrefix(jsonStr, "[") {
		return &ExecutionGraph{ImmediateResponse: jsonStr}, nil
	}

	// Handle cases where the LLM returns an array directly instead of an object wrapping "spans"
	if strings.HasPrefix(strings.TrimSpace(jsonStr), "[") {
		var spans []Span
		if err := json.Unmarshal([]byte(jsonStr), &spans); err != nil {
			return nil, fmt.Errorf("failed to parse orchestration plan from LLM output (expected JSON array): %w\nRaw output:\n%s", err, jsonStr)
		}
		graph.Spans = spans
	} else {
		if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil {
			return nil, fmt.Errorf("failed to parse orchestration plan from LLM output (expected JSON object): %w\nRaw output:\n%s", err, jsonStr)
		}
	}

	return &graph, nil
}

func (m *defaultSwarm) Reflect(ctx context.Context, prompt string, traj Trajectory) (*Reflection, error) {
	b, _ := json.MarshalIndent(traj.Spans, "", "  ")

	systemPrompt := `You are Swarm. Your job is to evaluate whether the user's original goal has been FULLY completed based on the execution trajectory.
If it is completed, set is_resolved to true.
If the agent only diagnosed a problem but did not physically apply the fix (e.g., using write_local_file), then the task is NOT resolved. You must set is_resolved to false and provide explicit next_steps to implement the fix.
If a sub-agent explicitly asked the user a question (e.g., "Do you approve this action?", "Which option do you prefer?"), or if the swarm is completely stuck and requires human intervention to proceed, you MUST set needs_user_input to true and return the question in the reasoning.

Additionally, you MUST identify any "timeless facts" discovered during this execution. A timeless fact is a piece of information that is true across sessions and projects (e.g., "The build command for this repo is X", "The secret ingredient is Y", "Tool Z is currently down").
If you find such facts, include them in the "new_facts" array. Be exhaustive but precise. Avoid ephemeral conversational state.

Output your response as strictly valid JSON matching this schema:
{
  "is_resolved": boolean,
  "needs_user_input": boolean,
  "reasoning": "string",
  "next_steps": "string",
  "new_facts": ["string"]
}`

	userPrompt := fmt.Sprintf("Original Goal: %s\n\nExecution Trajectory:\n%s", prompt, string(b))

	respIter := m.proModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(userPrompt, genai.Role("user"))},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system")),
			ResponseMIMEType:  "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"is_resolved":      {Type: genai.TypeBoolean, Description: "True if the original goal has been fully completed and verified."},
					"needs_user_input": {Type: genai.TypeBoolean, Description: "True if an agent explicitly asked the user a question or needs human intervention."},
					"reasoning":        {Type: genai.TypeString, Description: "Explanation for why the task is or isn't resolved. If needs_user_input is true, state the question here."},
					"next_steps":       {Type: genai.TypeString, Description: "Explicit instructions for what to do next if not resolved."},
					"new_facts": {
						Type:        genai.TypeArray,
						Items:       &genai.Schema{Type: genai.TypeString},
						Description: "Timeless facts discovered during this execution to be saved to semantic memory.",
					},
				},
				Required: []string{"is_resolved", "needs_user_input", "reasoning"},
			},
		},
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

	jsonStr = extractJSON(jsonStr)
	var reflection Reflection
	if err := json.Unmarshal([]byte(jsonStr), &reflection); err != nil {
		return nil, fmt.Errorf("failed to parse reflection from LLM output (expected JSON): %w\nRaw output:\n%s", err, jsonStr)
	}
	return &reflection, nil
}

func (m *defaultSwarm) SummarizeState(ctx context.Context, state string) (string, error) {
	prompt := fmt.Sprintf("You are an observer monitoring a swarm of AI agents. Here is their current activity:\n%s\n\nWrite a single, concise sentence (max 8 words) summarizing their overall progress to the user. Start with an action verb (e.g., 'Investigating the codebase', 'Running tests and debugging').", state)

	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
	}, false)

	for resp, err := range respIter {
		if err != nil {
			return "", err
		}
		if resp.Content != nil && len(resp.Content.Parts) > 0 {
			text := strings.TrimSpace(resp.Content.Parts[0].Text)
			text = strings.TrimPrefix(text, "\"")
			text = strings.TrimSuffix(text, "\"")
			return text, nil
		}
		break
	}
	return "Working...", nil
}

func (m *defaultSwarm) Chat(ctx context.Context, prompt string) (<-chan ObservableEvent, error) {
	out := make(chan ObservableEvent, 1000)
	// Record the user's initial prompt in the persistent session
	m.appendEvent(ctx, "user", prompt)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
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
		m.mu.RLock()
		lastAgentName := "None (Starting new conversation)"
		if m.lastAgent != "" {
			lastAgentName = m.lastAgent
		}

		var descriptions []string
		for _, name := range m.subAgentNames {
			if a, ok := m.agents[name]; ok {
				descriptions = append(descriptions, fmt.Sprintf("- **%s**: %s", name, a.Description()))
			}
		}
		specialistsList := strings.Join(descriptions, "\n")
		inputInstruction := m.inputInstruction
		m.mu.RUnlock()

		// Fetch and inject global conversation history for the Input Agent
		var inputHistoryParts []string
		if m.memory != nil && m.memory.Episodic() != nil {
			if hist, err := m.memory.Episodic().GetRecentHistory(ctx, m.sessionID, 5); err == nil {
				inputHistoryParts = hist
			}
		}

		dynamicInstruction := inputInstruction + fmt.Sprintf("\n\nAVAILABLE AGENTS:\n%s\n\nCURRENT CONTEXT: The last agent to respond was: %s.", specialistsList, lastAgentName)
		if len(inputHistoryParts) > 0 {
			// Only take the last 5 turns to keep the prompt small and fast
			if len(inputHistoryParts) > 5 {
				inputHistoryParts = inputHistoryParts[len(inputHistoryParts)-5:]
			}
			dynamicInstruction += "\n\nRECENT CONVERSATION HISTORY:\n" + strings.Join(inputHistoryParts, "\n")
		}

		// Inject relevant semantic memory facts for the Input Agent so it can short-circuit to Swarm if the answer is known
		if m.memory != nil && m.memory.Semantic() != nil {
			if facts, err := m.memory.Semantic().Retrieve(prompt, 3); err == nil && len(facts) > 0 {
				dynamicInstruction += "\n\n### RELEVANT SYSTEM FACTS (SEMANTIC MEMORY):\n" + strings.Join(facts, "\n")
			}
		}

		inputIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
			Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
			Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(dynamicInstruction, genai.Role("system"))},
		}, false)

		var inputResult string
		for resp, err := range inputIter {
			if err != nil {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateError, Error: fmt.Errorf("input classification failed: %w", err)}
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
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateExecuting, Thought: "Rerouting to Swarm…"}
			target = "swarm_agent"
		} else {
			target = "swarm_agent"
			m.mu.RLock()
			if m.lastAgent != "" {
				target = m.lastAgent
			}
			m.mu.RUnlock()
		}
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateComplete, FinalContent: "Routed to " + target}

		originalPrompt := prompt
		cyclePrompt := prompt

		maxCycles := 5
		for cycle := 0; cycle < maxCycles; cycle++ {
			var graph *ExecutionGraph
			var err error

			// 2. Swarm Coordination / Planning
			// If we are starting fresh or rerouting to Swarm Agent, we let it plan.
			if target == "swarm_agent" {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateThinking, Thought: "Analyzing request…"}
				planStart := time.Now()

				// Inject the same history parts into the planner if available
				plannerPrompt := cyclePrompt
				if cycle == 0 && len(inputHistoryParts) > 0 {
					plannerPrompt += "\n\n### RECENT CONVERSATION HISTORY (For Context):\n" + strings.Join(inputHistoryParts, "\n")
				}

				graph, err = m.Plan(ctx, plannerPrompt, o.GetTrajectory())
				if err != nil {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateError, Error: fmt.Errorf("coordination failed: %w", err)}
					return
				}

				if graph != nil && graph.ImmediateResponse == "" {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateComplete, FinalContent: "Delegated tasks to sub-agents."}
				}

				planJSON, _ := json.Marshal(graph)
				o.AddSpans(Span{
					ID: "coordination", Name: "Swarm Planning", Agent: "swarm_agent", Status: SpanStatusComplete,
					Kind:      SpanKindPlanner,
					StartTime: planStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
					Duration: time.Since(planStart).String(), Attributes: map[string]any{"gen_ai.prompt": cyclePrompt, "gen_ai.completion": string(planJSON)},
				})
			} else {
				// Direct execution with specialized agent (Node Autonomy)
				graph = &ExecutionGraph{Spans: []Span{{ID: "t1", Name: "Fulfill", Agent: target, Prompt: cyclePrompt}}}
			}

			// Namespace spans if this is a reflection cycle
			if cycle > 0 && graph != nil {
				prefix := fmt.Sprintf("c%d-", cycle)
				for i := range graph.Spans {
					graph.Spans[i].ID = prefix + graph.Spans[i].ID
					if graph.Spans[i].ParentID != "" {
						graph.Spans[i].ParentID = prefix + graph.Spans[i].ParentID
					}
					for j := range graph.Spans[i].Dependencies {
						graph.Spans[i].Dependencies[j] = prefix + graph.Spans[i].Dependencies[j]
					}
				}
			}

			// 3. Execution
			if len(graph.Spans) > 0 {
				events, updatedEngine, err := m.Execute(ctx, graph, o)
				if err != nil {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "execution", TaskName: "Graph Execution", State: AgentStateError, Error: fmt.Errorf("execution failed: %w", err)}
					return
				}
				for event := range events {
					out <- event
				}
				o = updatedEngine

				// 4. Reflect
				if target == "swarm_agent" {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateThinking, Thought: "Evaluating if original goal is complete..."}

					reflection, err := m.Reflect(ctx, originalPrompt, o.GetTrajectory())
					if err != nil {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateError, Error: fmt.Errorf("reflection failed: %w", err)}
						return
					}

					// Automatically commit any new facts discovered during reflection
					if m.memory != nil && m.memory.Semantic() != nil && len(reflection.NewFacts) > 0 {
						for _, fact := range reflection.NewFacts {
							_ = m.memory.Semantic().Commit(fact)
						}
					}

					// Passively compress the episodic working memory now that facts are extracted
					o.Prune()

					if reflection.NeedsUserInput {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateComplete, Thought: "Waiting for user input.", FinalContent: reflection.Reasoning}
						break
					} else if reflection.IsResolved {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateComplete, Thought: "Goal satisfied."}
						break
					} else {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateComplete, Thought: "Goal not satisfied yet. Replanning."}
						cyclePrompt = fmt.Sprintf("Original Goal: %s\n\nReflection from last cycle: %s\nNext Steps: %s", originalPrompt, reflection.Reasoning, reflection.NextSteps)
					}
				} else {
					break // Direct routes exit immediately
				}
			} else if graph.ImmediateResponse != "" {
				if m.runOutputAgent(ctx, out, o, "Swarm", graph.ImmediateResponse) {
					// Record the immediate response in the persistent session
					m.appendEvent(ctx, "model", graph.ImmediateResponse)
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateComplete, FinalContent: graph.ImmediateResponse}
				}
				break
			} else {
				break
			}
		}

		// 5. End of Chat
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

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
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
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateError, Error: fmt.Errorf("deadlock detected in execution graph")}
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

				if isDeadlocked, reason := o.IsDeadlocked(); isDeadlocked {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Overwatch", State: AgentStateError, Error: fmt.Errorf("deadlock detected: %s", reason)}

					// Automatically commit this failure to semantic memory so it persists across sessions
					if m.memory != nil && m.memory.Semantic() != nil {
						_ = m.memory.Semantic().Commit(fmt.Sprintf("TOOL FAILURE (Overwatch): %s", reason))
					}

					// Force a hard replan and explicitly tell the planner WHY it failed so it routes around the block
					done.Replan = true
					done.Result = fmt.Sprintf("CRITICAL SYSTEM ERROR (OVERWATCH): %s You MUST find an alternative approach or ask the user for help.", reason)
					// reset the deadlock flag so the new plan can execute
					o.mu.Lock()
					o.deadlocked = false
					o.deadlockReason = ""
					o.mu.Unlock()
				}

				// Handle dynamic replanning or subgraph expansion
				if done.Replan {
					if replanCount >= maxReplans {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateError, Error: fmt.Errorf("maximum replan attempts reached, halting loop")}
						continue
					}
					replanCount++
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateThinking, Thought: fmt.Sprintf("Replanning effort %d/%d…", replanCount, maxReplans)}

					wg.Add(1)
					go func(failureResult string) {
						defer wg.Done()
						newG, err := m.Plan(ctx, "Pivot: "+failureResult, o.GetTrajectory())
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
					}(done.Result)
				}
			}
		}

		// Wait for all remaining active spans to recognize the context cancellation and gracefully exit
		wg.Wait()
	}()

	return out, o, nil
}

func (m *defaultSwarm) executeSpan(ctx context.Context, out chan<- ObservableEvent, o *Engine, span Span) (string, SpanStatus, bool) {
	m.mu.RLock()
	targetAgent, ok := m.agents[span.Agent]
	m.mu.RUnlock()
	
	if !ok {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("agent not found: %s", span.Agent)}
		return "Agent not found", SpanStatusFailed, false
	}

	// Use a unique session ID for this specific span
	spanSessionID := fmt.Sprintf("%s/%s", m.sessionID, span.ID)

	// To prevent SQLite "database is locked" errors under massive concurrency,
	// we use a lightweight in-memory session service for the sub-span runner.
	// The agent's interactions with the global blackboard (write_state tool)
	// will still correctly route to the global m.sessionSvc.
	spanSessionSvc := session.InMemoryService()

	_, err := spanSessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   "swarm-cli",
		UserID:    m.userID,
		SessionID: spanSessionID,
	})
	if err != nil {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("failed to initialize session for span: %w", err)}
		return "Session initialization failed", SpanStatusFailed, false
	}

	spanRunner, _ := runner.New(runner.Config{
		AppName:        "swarm-cli",
		Agent:          targetAgent,
		SessionService: spanSessionSvc,
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

		var obsRunning bool
		var obsMu sync.Mutex

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
					obsMu.Lock()
					if obsRunning {
						obsMu.Unlock()
						timer.Reset(interval)
						continue
					}
					obsRunning = true
					obsMu.Unlock()

					newLines = false

					histCopy := make([]string, len(history))
					copy(histCopy, history)
					obsWg.Add(1)
					go func(hist []string) {
						defer obsWg.Done()
						defer func() {
							obsMu.Lock()
							obsRunning = false
							obsMu.Unlock()
						}()
						obsCtx, obsCancel := context.WithTimeout(spanCtx, 15*time.Second) // Fix: Tie observer timeout directly to spanCtx
						defer obsCancel()

						obsPrompt := fmt.Sprintf("Monitor: %s. Recent Telemetry:\n%s\n\nTask: Output a concise 3-8 word phrase summarizing the current activity (e.g., 'Searching for authentication logic...' or 'Running unit tests...'). If you detect an infinite loop or severe error, output 'INTERVENE: [reason]'.", targetAgent.Name(), strings.Join(hist, "\n"))

						respIter := m.fastModel.GenerateContent(obsCtx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(obsPrompt, genai.Role("user"))}}, false)
						for resp, err := range respIter {
							if err != nil {
								out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Observer", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateWaiting, ObserverSummary: "Telemetry summary failed: " + err.Error()}
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

	promptStr := fmt.Sprintf("TASK: %s\nINSTRUCTIONS: %s", span.Name, span.Prompt)
	promptStr += fmt.Sprintf("\n\n### TASK CONTEXT\nYour current Task ID is: %s\nIf you need to spawn subtasks, use this ID as their 'parent_id' or explicitly in their 'dependencies' list to ensure they block this task's completion if necessary.", span.ID)

	// Inject relevant semantic memory facts
	if m.memory != nil && m.memory.Semantic() != nil {
		if facts, err := m.memory.Semantic().Retrieve(span.Prompt, 3); err == nil && len(facts) > 0 {
			promptStr += "\n\n### RELEVANT MEMORY FACTS:\n" + strings.Join(facts, "\n")
		}
	}

	if len(stateParts) > 0 {
		promptStr = promptStr + "\n\n### SESSION STATE\n" + strings.Join(stateParts, "\n")
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
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("error encountered: %w", err)}
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
					var pgid int
					if val, ok := part.FunctionResponse.Response["pgid"]; ok {
						if num, ok := val.(float64); ok {
							pgid = int(num)
						} else if num, ok := val.(int); ok {
							pgid = num
						}
					}
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, ToolName: part.FunctionResponse.Name, PGID: pgid}

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
	m.mu.Lock()
	m.lastAgent = targetAgent.Name()
	m.mu.Unlock()

	// If there was a hard error, skip Output Agent and immediately return with failed status and replan flag
	if strings.Contains(finalText, "\n\nERROR: ") || strings.Contains(finalText, "command not found") {
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
	if m.memory != nil && m.memory.Episodic() != nil {
		_ = m.memory.Episodic().AppendEvent(ctx, m.sessionID, author, content)
	}
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
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Output Agent", State: AgentStateError, FinalContent: "Rejected: " + res}
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
	baseDir, err := GetConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(baseDir, m.trajectoryDir)
	_ = os.MkdirAll(dir, 0o755)

	filename := fmt.Sprintf("%s.json", traj.TraceID)
	if m.sessionID != "" {
		filename = fmt.Sprintf("%s_%s.json", m.sessionID, traj.TraceID)
	}
	// Sanitize output path
	filename = strings.ReplaceAll(filename, "/", "_")
	path := filepath.Join(dir, filename)

	if m.telemetryConfigured || m.forceDonate {
		// Convert the typed struct into a generic map so we can inject the "donate" field at the root level without altering the core schema
		b, err := json.Marshal(traj)
		if err == nil {
			var dynMap map[string]interface{}
			if err := json.Unmarshal(b, &dynMap); err == nil {
				dynMap["donate"] = true
				b, _ = json.MarshalIndent(dynMap, "", "  ")
				_ = os.WriteFile(path, b, 0o600)
				return
			}
		}
	}

	b, _ := json.MarshalIndent(traj, "", "  ")
	_ = os.WriteFile(path, b, 0o600)
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
