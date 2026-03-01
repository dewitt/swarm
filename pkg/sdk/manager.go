package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
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

type ChatEventType string
const (
	ChatEventHandoff ChatEventType = "handoff"; ChatEventToolCall ChatEventType = "tool_call"; ChatEventToolResult ChatEventType = "tool_result"
	ChatEventTelemetry ChatEventType = "telemetry"; ChatEventThought ChatEventType = "thought"; ChatEventObserver ChatEventType = "observer"
	ChatEventReplan ChatEventType = "replan"; ChatEventDebug ChatEventType = "debug"; ChatEventFinalResponse ChatEventType = "final_response"; ChatEventError ChatEventType = "error"
)
type ChatEvent struct { Type ChatEventType; Agent, TaskID, Content string }

type AgentManager interface {
	Discover(ctx context.Context, dir string) (*AgentManifest, error)
	AddContext(path string) error
	DropContext(path string)
	ListContext() []string
	Plan(ctx context.Context, prompt string) (*ExecutionGraph, error)
	Execute(ctx context.Context, g *ExecutionGraph, o *Orchestrator) (<-chan ChatEvent, *Orchestrator, error)
	Chat(ctx context.Context, prompt string) (<-chan ChatEvent, error)
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

type defaultManager struct {
	run              *runner.Runner
	db               *gorm.DB
	sessionSvc       session.Service
	userID           string
	sessionID        string
	skills           []*Skill
	clientCfg        *genai.ClientConfig
	pinnedContext    map[string]string
	inputInstruction    string
	outputInstruction   string
	planningInstruction string
	debugMode           bool
	flashModel        model.LLM
	proModel          model.LLM
	fastModel         model.LLM
	toolRegistry      map[string]tool.Tool
	inputAgent        agent.Agent
	subAgentNames     []string
	agents            map[string]agent.Agent
	lastAgent         string // Tracks the last agent to respond
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

func (m *defaultManager) IsDebug() bool { return m.debugMode }
func (m *defaultManager) SetDebug(enabled bool) { m.debugMode = enabled }

func (m *defaultManager) Explain(ctx context.Context, traj Trajectory) (string, error) {
	trajJSON, _ := json.MarshalIndent(traj, "", "  ")
	prompt := fmt.Sprintf("You are the Swarm Historian. Explain WHY this path was taken. Concisely. Trajectory: %s", string(trajJSON))
	respIter := m.proModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}}, false)
	var exp string; for resp, err := range respIter { if err != nil { return "", err }; if resp.Content != nil && len(resp.Content.Parts) > 0 { exp += resp.Content.Parts[0].Text } }
	return strings.TrimSpace(exp), nil
}

func (m *defaultManager) Reload() error {
	var subAgents []agent.Agent
	var loadedSkills []*Skill
	
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
		skill, err := LoadSkill(dir)
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
		instruction := skill.Instructions
		if skill.Manifest.Name != "input_agent" && skill.Manifest.Name != "output_agent" && skill.Manifest.Name != "swarm_agent" && skill.Manifest.Name != "planning_agent" {
			instruction += "\n\nSUB-AGENT MODE: You are being invoked by the Swarm Agent to perform a specific task. Skip all greetings and introductory talk. Focus ONLY on executing the task and providing the results."
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
	skill, _ := LoadSkill(filepath.Join(skillsPath, "swarm-agent"))
	instruction := fmt.Sprintf(skill.Instructions, specialistsList)
	swarmAgent, _ = llmagent.New(llmagent.Config{
		Name:        "swarm_agent",
		Model:       m.fastModel,
		Instruction: instruction,
		Tools:       []tool.Tool{m.toolRegistry["list_local_files"], m.toolRegistry["read_local_file"], m.toolRegistry["grep_search"]},
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
	var jsonStr string; for resp, err := range respIter { if err != nil { return nil, err }; if resp.Content != nil && len(resp.Content.Parts) > 0 { jsonStr += resp.Content.Parts[0].Text } }
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "DEEP_PLAN_REQUIRED" {
		respIter = m.proModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}, Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system"))}}, false)
		jsonStr = ""; for resp, err := range respIter { if err != nil { return nil, err }; if resp.Content != nil && len(resp.Content.Parts) > 0 { jsonStr += resp.Content.Parts[0].Text } }; jsonStr = strings.TrimSpace(jsonStr)
	}
	re := regexp.MustCompile("(?s)\\{.*\\}"); match := re.FindString(jsonStr); if match != "" { jsonStr = match }
	var graph ExecutionGraph; if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil { return nil, err }; return &graph, nil
}

func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan ChatEvent, error) {
	out := make(chan ChatEvent, 1000)
	// Record the user's initial prompt in the persistent session
	m.appendEvent(ctx, "user", prompt)

	go func() {
		defer close(out)
		swarmStartTime := time.Now()
		o := NewOrchestrator(nil)

		// 1. Input Classification (Fast Path / CIA)
		inputStart := time.Now()
		out <- ChatEvent{Type: ChatEventThought, Agent: "Input Agent", Content: "Classifying intent…"}

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
				out <- ChatEvent{Type: ChatEventError, Content: "Input classification failed: " + err.Error()}
				return
			}
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				inputResult = resp.Content.Parts[0].Text
			}
			break
		}

		o.AddTasks(Task{
			ID: "input", Name: "Classification", Agent: "input_agent", Status: TaskStatusComplete,
			Kind: SpanKindPlanner,
			StartTime: inputStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
			Duration: time.Since(inputStart).String(), Attributes: map[string]any{"gen_ai.prompt": prompt, "gen_ai.completion": inputResult},
		})

		var target string
		if strings.HasPrefix(inputResult, "ROUTE TO: swarm_agent") {
			out <- ChatEvent{Type: ChatEventThought, Agent: "Input Agent", Content: "Rerouting to Swarm Agent…"}
			target = "swarm_agent"
		} else {
			target = "swarm_agent"
			if m.lastAgent != "" {
				target = m.lastAgent
			}
		}

		var graph *ExecutionGraph
		var err error

		// 2. Swarm Coordination / Planning
		// If we are starting fresh or rerouting to Swarm Agent, we let it plan.
		if target == "swarm_agent" {
			out <- ChatEvent{Type: ChatEventThought, Agent: "Swarm Agent", Content: "Analyzing request…"}
			planStart := time.Now()
			graph, err = m.Plan(ctx, prompt)
			if err != nil {
				out <- ChatEvent{Type: ChatEventError, Content: "Coordination failed: " + err.Error()}
				return
			}
			planJSON, _ := json.Marshal(graph)
			o.AddTasks(Task{
				ID: "coordination", Name: "Swarm Planning", Agent: "swarm_agent", Status: TaskStatusComplete,
				Kind: SpanKindPlanner,
				StartTime: planStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
				Duration: time.Since(planStart).String(), Attributes: map[string]any{"gen_ai.prompt": prompt, "gen_ai.completion": string(planJSON)},
			})
		} else {
			// Direct execution with specialized agent (Node Autonomy)
			graph = &ExecutionGraph{Tasks: []Task{{ID: "t1", Name: "Fulfill", Agent: target, Prompt: prompt}}}
		}

		// 3. Execution
		if graph.ImmediateResponse != "" {
			if m.runOutputAgent(ctx, out, o, "Swarm Agent", graph.ImmediateResponse) {
				// Record the immediate response in the persistent session
				m.appendEvent(ctx, "model", graph.ImmediateResponse)
				out <- ChatEvent{Type: ChatEventFinalResponse, Agent: "Swarm Agent", Content: graph.ImmediateResponse}
			}
		} else {
			events, _, err := m.Execute(ctx, graph, o)
			if err != nil {
				out <- ChatEvent{Type: ChatEventError, Content: "Execution failed: " + err.Error()}
				return
			}
			for event := range events {
				out <- event
			}
		}

		// 4. Debug / Trajectory
		if m.debugMode {
			traj := o.GetTrajectory()
			traj.TotalDuration = time.Since(swarmStartTime).String()
			b, _ := json.MarshalIndent(traj, "", "  ")
			out <- ChatEvent{Type: ChatEventDebug, Agent: "Swarm", Content: string(b)}
		}
	}()
	return out, nil
}


func (m *defaultManager) Execute(ctx context.Context, g *ExecutionGraph, o *Orchestrator) (<-chan ChatEvent, *Orchestrator, error) {
	if o == nil {
		o = NewOrchestrator(g)
	}
	out := make(chan ChatEvent, 1000)
	if g != nil {
		o.AddTasks(g.Tasks...)
	}

	go func() {
	defer close(out)
	type completedTask struct {
		ID     string
		Result string
		Status TaskStatus
		Replan bool
	}
	completed := make(chan completedTask, 100)
	activeTasks := 0
	var wg sync.WaitGroup
	replanCount := 0
	const maxReplans = 3

	for {
		ready := o.GetReadyTasks()
		for _, t := range ready {
			o.MarkActive(t.ID)
			activeTasks++
			wg.Add(1)
			go func(task Task) {
				defer wg.Done()
				result, status, needsReplan := m.executeTask(ctx, out, o, task)
				completed <- completedTask{ID: task.ID, Result: result, Status: status, Replan: needsReplan}
			}(t)
		}

		if activeTasks == 0 {
			if o.IsComplete() {
				break
			} else {
				out <- ChatEvent{Type: ChatEventError, Agent: "Swarm", Content: "Deadlock detected in execution graph."}
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case done := <-completed:
			if done.Status == TaskStatusComplete {
				o.MarkComplete(done.ID, done.Result)
			} else {
				o.MarkFailed(done.ID)
			}
			activeTasks--
				// Handle dynamic replanning or subgraph expansion
				if done.Replan {
					if replanCount >= maxReplans {
						out <- ChatEvent{Type: ChatEventError, Agent: "Swarm", Content: "Maximum replan attempts reached. Halting loop."}
						continue
					}
					replanCount++
					out <- ChatEvent{Type: ChatEventThought, Agent: "Swarm", Content: fmt.Sprintf("Replanning effort %d/%d…", replanCount, maxReplans)}
					newG, err := m.Plan(ctx, "Pivot: "+done.Result)
					if err == nil {
						o.AddTasks(newG.Tasks...)
					}
				} else if strings.Contains(done.Result, "```json") && strings.Contains(done.Result, "\"tasks\":") {
					// Strict check for JSON blocks to avoid false positives in markdown text
					re := regexp.MustCompile("(?s)```json\n(\\{.*\\})\n```")
					match := re.FindStringSubmatch(done.Result)
					if len(match) > 1 {
						var subGraph ExecutionGraph
						if err := json.Unmarshal([]byte(match[1]), &subGraph); err == nil {
							for i := range subGraph.Tasks {
								subGraph.Tasks[i].ParentID = done.ID
								subGraph.Tasks[i].ID = fmt.Sprintf("%s_%s", done.ID, subGraph.Tasks[i].ID)
							}
							o.AddTasks(subGraph.Tasks...)
						}
					}
				}
			}
		}
		wg.Wait()
	}()

	return out, o, nil
}

func (m *defaultManager) executeTask(ctx context.Context, out chan<- ChatEvent, o *Orchestrator, task Task) (string, TaskStatus, bool) {
	targetAgent, ok := m.agents[task.Agent]
	if !ok {
		out <- ChatEvent{Type: ChatEventError, Agent: "Swarm", TaskID: task.ID, Content: "Agent not found: " + task.Agent}
		return "Agent not found", TaskStatusFailed, false
	}

	// Use a unique session ID for this specific task to prevent turn-order corruption
	// in the shared session service, especially during parallel execution.
	taskSessionID := fmt.Sprintf("%s/%s", m.sessionID, task.ID)
	
	// Ensure the task session exists in the database
	_, _ = m.sessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   "swarm-cli",
		UserID:    m.userID,
		SessionID: taskSessionID,
	})

	taskRunner, _ := runner.New(runner.Config{
		AppName:        "swarm-cli",
		Agent:          targetAgent,
		SessionService: m.sessionSvc,
	})

	out <- ChatEvent{Type: ChatEventThought, Agent: targetAgent.Name(), TaskID: task.ID, Content: "Starting…"}

	// Telemetry and Observation
	telemetryChan := make(chan string, 100)
	taskCtx, cancel := context.WithCancel(context.WithValue(ctx, telemetryContextKey{}, (chan<- string)(telemetryChan)))
	defer cancel()

	telemetryDone := make(chan struct{})
	var obsWg sync.WaitGroup
	obsWg.Add(1)
	go func() {
		defer close(telemetryDone)
		defer obsWg.Done()
		var lastLines []string
		for line := range telemetryChan {
			out <- ChatEvent{Type: ChatEventTelemetry, Agent: targetAgent.Name(), TaskID: task.ID, Content: line}
			lastLines = append(lastLines, line)
			if len(lastLines) >= 10 {
				obsWg.Add(1)
				go func(history []string) {
					defer obsWg.Done()
					obsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
					obsPrompt := fmt.Sprintf("Monitor: %s. Telemetry: %s. If loop output INTERVENE: [reason]. Else OK.", targetAgent.Name(), strings.Join(history, "\n"))
					respIter := m.fastModel.GenerateContent(obsCtx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(obsPrompt, genai.Role("user"))}}, false)
					for resp, err := range respIter {
						if err == nil && resp.Content != nil && len(resp.Content.Parts) > 0 {
							text := resp.Content.Parts[0].Text
							if strings.HasPrefix(text, "INTERVENE:") {
								out <- ChatEvent{Type: ChatEventObserver, Agent: "Observer", TaskID: task.ID, Content: text}
								cancel() // Forcefully halt the agent interaction loop
							}
						}
						break
					}
				}(lastLines)
				lastLines = nil
			}
		}
	}()

	// Inject results from dependencies into the prompt (Node Autonomy)
	contextMap := o.GetContext()
	var contextParts []string
	for _, depID := range task.Dependencies {
		if res, ok := contextMap[depID]; ok {
			contextParts = append(contextParts, fmt.Sprintf("Output from previous task (%s):\n%s", depID, res))
		}
	}

	// Fetch and inject global conversation history
	var historyParts []string
	if sessResp, err := m.sessionSvc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID}); err == nil {
		for ev := range sessResp.Session.Events().All() {
			if ev.Content != nil {
				for _, part := range ev.Content.Parts {
					if part.Text != "" {
						historyParts = append(historyParts, fmt.Sprintf("[%s]: %s", ev.Author, part.Text))
					}
				}
			}
		}
	}

	promptStr := fmt.Sprintf("TASK: %s\nINSTRUCTIONS: %s", task.Name, task.Prompt)
	
	if len(historyParts) > 0 {
		promptStr = promptStr + "\n\n### CONVERSATION HISTORY (FOR CONTEXT)\n" + strings.Join(historyParts, "\n")
	}
	if len(contextParts) > 0 {
		promptStr = promptStr + "\n\n### RESULTS FROM PREVIOUS TASKS\n" + strings.Join(contextParts, "\n\n")
	}

	// Execute with the unique taskSessionID
	events := taskRunner.Run(taskCtx, m.userID, taskSessionID, genai.NewContentFromText(promptStr, genai.Role("user")), agent.RunConfig{})

	var full strings.Builder
	var needsReplan bool
	activeToolSpans := make(map[string][]Task)

	for event, err := range events {
		if err != nil {
			out <- ChatEvent{Type: ChatEventError, Agent: targetAgent.Name(), TaskID: task.ID, Content: err.Error()}
			break
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.FunctionCall != nil {
					if part.FunctionCall.Name == "request_replan" {
						needsReplan = true
					}
					out <- ChatEvent{Type: ChatEventToolCall, Agent: targetAgent.Name(), TaskID: task.ID, Content: part.FunctionCall.Name}
					
					// Record tool span start
					toolStart := time.Now()
					toolID := fmt.Sprintf("%s-tool-%s-%d", task.ID, part.FunctionCall.Name, toolStart.UnixNano())
					toolTask := Task{
						ID: toolID, ParentID: task.ID, TraceID: task.TraceID,
						Name: part.FunctionCall.Name, Kind: SpanKindTool, Status: TaskStatusActive,
						StartTime: toolStart.Format(time.RFC3339Nano),
						Attributes: map[string]any{"gen_ai.tool_args": part.FunctionCall.Args},
					}
					o.AddTasks(toolTask)
					activeToolSpans[part.FunctionCall.Name] = append(activeToolSpans[part.FunctionCall.Name], toolTask)
				}
				if part.FunctionResponse != nil {
					out <- ChatEvent{Type: ChatEventToolResult, Agent: targetAgent.Name(), TaskID: task.ID, Content: part.FunctionResponse.Name}
					
					// Record tool span completion (pop from queue)
					if tasks, ok := activeToolSpans[part.FunctionResponse.Name]; ok && len(tasks) > 0 {
						t := tasks[0]
						activeToolSpans[part.FunctionResponse.Name] = tasks[1:]
						
						now := time.Now()
						t.Status = TaskStatusComplete
						t.EndTime = now.Format(time.RFC3339Nano)
						if t.StartTime != "" {
							start, _ := time.Parse(time.RFC3339Nano, t.StartTime)
							t.Duration = now.Sub(start).String()
						}
						resJSON, _ := json.Marshal(part.FunctionResponse.Response)
						t.Attributes["gen_ai.tool_result"] = string(resJSON)
						o.AddTasks(t) // Update the task in the orchestrator
					}
				}
				if part.Text != "" && !part.Thought {
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
	for _, tasks := range activeToolSpans {
		for _, t := range tasks {
			now := time.Now()
			t.Status = TaskStatusFailed
			t.EndTime = now.Format(time.RFC3339Nano)
			t.Attributes["gen_ai.tool_result"] = "Tool execution interrupted or incomplete"
			o.AddTasks(t)
		}
	}

	if finalText == "" {
		return "No response emitted by agent.", TaskStatusFailed, false
	}

	// Update the global state of the last agent to respond
	m.lastAgent = targetAgent.Name()

	// Output Sanity Check (Output Agent)
	if m.runOutputAgent(ctx, out, o, targetAgent.Name(), finalText) {
		// Record the response in the main conversation history
		m.appendEvent(ctx, "model", fmt.Sprintf("[%s]: %s", targetAgent.Name(), finalText))
		out <- ChatEvent{Type: ChatEventFinalResponse, Agent: targetAgent.Name(), TaskID: task.ID, Content: finalText}
		return finalText, TaskStatusComplete, needsReplan
	}

	out <- ChatEvent{Type: ChatEventThought, Agent: "Swarm", TaskID: task.ID, Content: "Output Agent rejected the response as problematic."}
	return "Output Agent rejected response.", TaskStatusComplete, true
}



func (m *defaultManager) appendEvent(ctx context.Context, author, content string) {
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

func (m *defaultManager) runOutputAgent(ctx context.Context, out chan<- ChatEvent, o *Orchestrator, agentName, content string) bool {
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
				out <- ChatEvent{Type: ChatEventThought, Agent: "Output Agent", Content: "Rejected: " + res}
				approved = false
			}
		}
		break
	}

	// Update UI card state
	if approved {
		out <- ChatEvent{Type: ChatEventFinalResponse, Agent: "Output Agent", Content: "OK"}
	}

	if o != nil {
		o.AddTasks(Task{
			ID: "output-agent-" + fmt.Sprintf("%d", coaStart.UnixNano()), Name: "Sanity Check", Agent: "output_agent",
			Status: TaskStatusComplete, StartTime: coaStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
			Duration: time.Since(coaStart).String(), Attributes: map[string]any{"gen_ai.prompt": coaPrompt, "gen_ai.completion": res},
		})
	}
	return approved
}

func (m *defaultManager) Rewind(n int) error {
	if n <= 0 { return nil }
	var evs []struct{ Timestamp string }; err := m.db.Table("events").Select("timestamp").Where("session_id = ? AND author = ?", m.sessionID, "user").Order("timestamp DESC").Limit(n).Find(&evs).Error
	if err != nil { return err }; if len(evs) < n { return m.db.Table("events").Where("session_id = ?", m.sessionID).Delete(nil).Error }
	return m.db.Table("events").Where("session_id = ? AND timestamp >= ?", m.sessionID, evs[len(evs)-1].Timestamp).Delete(nil).Error
}
