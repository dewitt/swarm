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
	LSP() *ManagedLSP // Provides access to the language server bridge
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
	lsp                 *ManagedLSP
}

type SwarmConfig struct {
	Model             model.LLM
	ResumeLastSession bool
	Debug             bool
	DatabaseURI       string
	TrajectoryDir     string // Optional override for where trajectories are saved (e.g., "eval_trajectories")
	ForceDonate       bool           // If true, forces the "donate" telemetry flag to true for this run
	LSPCommand        string         // Optional command to start an MCP-compatible LSP server
	LSPArgs           []string       // Arguments for the LSP command
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

	if len(cfg) > 0 && cfg[0].LSPCommand != "" {
		m.lsp = NewManagedLSP(cfg[0].LSPCommand, cfg[0].LSPArgs...)
		// We start it asynchronously so it doesn't block the UI boot
		go func() {
			_ = m.lsp.Start(context.Background())
		}()
	}

	readStateTool, _ := functiontool.New(functiontool.Config{Name: "read_state"}, m.readState)
	writeStateTool, _ := functiontool.New(functiontool.Config{Name: "write_state"}, m.writeState)
	retrieveFactTool, _ := functiontool.New(functiontool.Config{Name: "retrieve_fact"}, m.retrieveFact)
	spawnSubtaskTool, _ := functiontool.New(functiontool.Config{Name: "spawn_subtask"}, m.spawnSubtask)
	codeSkeletonTool, _ := functiontool.New(functiontool.Config{Name: "get_code_skeleton"}, getCodeSkeletonTool)
	analyzeImpactTool, _ := functiontool.New(functiontool.Config{Name: "analyze_impact"}, m.analyzeImpact)
	getAPISignatureTool, _ := functiontool.New(functiontool.Config{Name: "get_api_signature"}, m.getAPISignature)
	validateCodeTool, _ := functiontool.New(functiontool.Config{Name: "validate_code"}, m.validateCode)
	renameSymbolTool, _ := functiontool.New(functiontool.Config{Name: "rename_symbol"}, m.renameSymbol)

	m.toolRegistry = map[string]tool.Tool{
		"list_local_files": listTool, "read_local_file": readTool, "grep_search": grepTool, "write_local_file": writeTool, "git_commit": gitCommit, "git_push": gitPush, "bash_execute": bashExecute, "web_fetch": webFetchTool, "google_search": googleSearchTool, "request_replan": replanTool,
		"read_state": readStateTool, "write_state": writeStateTool, "retrieve_fact": retrieveFactTool, "spawn_subtask": spawnSubtaskTool, "get_code_skeleton": codeSkeletonTool,
		"analyze_impact": analyzeImpactTool, "get_api_signature": getAPISignatureTool, "validate_code": validateCodeTool, "rename_symbol": renameSymbolTool,
	}

	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
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

func (m *defaultSwarm) LSP() *ManagedLSP { return m.lsp }

func (m *defaultSwarm) Close() error {
	if m.lsp != nil {
		_ = m.lsp.Close()
	}
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
