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
	inputInstruction string
	outputInstruction string
	debugMode         bool
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
	var subAgents []agent.Agent; var loadedSkills []*Skill; skillDirs := []string{}
	entries, _ := os.ReadDir("skills"); for _, entry := range entries { if entry.IsDir() { skillDirs = append(skillDirs, filepath.Join("skills", entry.Name())) } }
	for _, dir := range skillDirs {
		skill, err := LoadSkill(dir); if err != nil { continue }
		loadedSkills = append(loadedSkills, skill); var skillTools []tool.Tool; skillTools = append(skillTools, m.toolRegistry["request_replan"])
		for _, toolName := range skill.Manifest.Tools { if t, ok := m.toolRegistry[toolName]; ok { skillTools = append(skillTools, t) } }
		targetModel := m.proModel; if skill.Manifest.Model == "flash" { targetModel = m.flashModel }
		skillAgent, _ := llmagent.New(llmagent.Config{Name: skill.Manifest.Name, Model: targetModel, Description: skill.Manifest.Description, Instruction: skill.Instructions, Tools: skillTools})
		subAgents = append(subAgents, skillAgent)
	}
	var subAgentNames []string; for _, sa := range subAgents { subAgentNames = append(subAgentNames, sa.Name()) }
	swarmInstruction := fmt.Sprintf("You are the Swarm Agent. Persona of this app. NODE AUTONOMY: Respond directly or orchestrate. Use Planning Agent for complex graphs. Sub-agents: %s", strings.Join(subAgentNames, ", "))
	swarmAgent, _ := llmagent.New(llmagent.Config{Name: "swarm_agent", Model: m.flashModel, Instruction: swarmInstruction, Tools: []tool.Tool{m.toolRegistry["list_local_files"], m.toolRegistry["read_local_file"], m.toolRegistry["grep_search"]}, SubAgents: subAgents})
	inputInstruction := fmt.Sprintf(`You are the Input Agent.
Your job is to take the user's keystrokes and classify their intent.
Available agents: %s

TRIVIAL QUERIES: If the user input is a greeting (e.g. "Hello"), social inquiry, or meta-question about the app, you MUST output: "ROUTE TO: swarm_agent".

ROUTING RULES:
- For a new request, digression, or greeting, output: "ROUTE TO: swarm_agent"
- For a specific technical task matching a sub-agent, output: "ROUTE TO: [agent_name]"
- Otherwise, output: "CONTINUE"

CRITICAL: ONLY output the ROUTE line. Do NOT output "ROUTE TO: None" or any other text. Be instant.`, strings.Join(subAgentNames, ", "))

	inputAgent, _ := llmagent.New(llmagent.Config{Name: "input_agent", Model: m.fastModel, Instruction: inputInstruction})
	planningAgent, _ := llmagent.New(llmagent.Config{Name: "planning_agent", Model: m.proModel, Instruction: "Plan DAGs."})
	outputInstruction := "You are the Output Agent. Your job is to sanity check responses before they are shown to the human. Output ONLY 'OK' or 'FIX: [reason]'. Do not be helpful. Do not explain. Just OK or FIX."
	outputAgent, _ := llmagent.New(llmagent.Config{Name: "output_agent", Model: m.fastModel, Instruction: outputInstruction})
	m.run, _ = runner.New(runner.Config{AppName: "swarm-cli", Agent: swarmAgent, SessionService: m.sessionSvc})
	m.skills = loadedSkills; m.subAgentNames = subAgentNames; m.inputInstruction = inputInstruction; m.inputAgent = inputAgent; m.outputInstruction = outputInstruction
	m.agents = make(map[string]agent.Agent); m.agents["swarm_agent"] = swarmAgent; m.agents["input_agent"] = inputAgent; m.agents["planning_agent"] = planningAgent; m.agents["output_agent"] = outputAgent
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
	allAgents := append(m.subAgentNames, "swarm_agent")
	systemPrompt := fmt.Sprintf(`You are the Planning Agent. Decompose request into DAG JSON. SCHEMA: { "tasks": [{ "id": "t1", "name": "N", "agent": "A", "prompt": "P", "dependencies": [] }] }. GREETINGS: use immediate_response. COMPLEX: DEEP_PLAN_REQUIRED. AVAILABLE: %s. RULE: Use EXACT field names. NO markdown.`, strings.Join(allAgents, ", "))
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))}, Config: &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system"))}}, false)
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

		var graph *ExecutionGraph
		var err error

		if strings.HasPrefix(inputResult, "ROUTE TO: ") {
			target := strings.TrimSpace(strings.TrimPrefix(inputResult, "ROUTE TO: "))
			if target != "" {
				out <- ChatEvent{Type: ChatEventThought, Agent: "Input Agent", Content: "Routing directly to " + target}
				graph = &ExecutionGraph{Tasks: []Task{{ID: "t1", Name: "Fulfill", Agent: target, Prompt: prompt}}}
			}
		}

		// 2. Planning (if no direct route)
		if graph == nil {
			out <- ChatEvent{Type: ChatEventThought, Agent: "Planning Agent", Content: "Generating execution plan…"}
			planStart := time.Now()
			graph, err = m.Plan(ctx, prompt)
			if err != nil {
				out <- ChatEvent{Type: ChatEventError, Content: "Planning failed: " + err.Error()}
				return
			}
			planJSON, _ := json.Marshal(graph)
			o.AddTasks(Task{
				ID: "planning", Name: "Graph Generation", Agent: "planning_agent", Status: TaskStatusComplete,
				Kind: SpanKindPlanner,
				StartTime: planStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
				Duration: time.Since(planStart).String(), Attributes: map[string]any{"gen_ai.prompt": prompt, "gen_ai.completion": string(planJSON)},
			})
		}

		// 3. Execution
		if graph.ImmediateResponse != "" {
			if m.runOutputAgent(ctx, out, o, "Planning Agent", graph.ImmediateResponse) {
				out <- ChatEvent{Type: ChatEventFinalResponse, Agent: "Planning Agent", Content: graph.ImmediateResponse}
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
			out <- ChatEvent{Type: ChatEventDebug, Agent: "Orchestrator", Content: string(b)}
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
			Replan bool
		}
		completed := make(chan completedTask, 100)
		activeTasks := 0
		var wg sync.WaitGroup

		for {
			ready := o.GetReadyTasks()
			for _, t := range ready {
				o.MarkActive(t.ID)
				activeTasks++
				wg.Add(1)
				go func(task Task) {
					defer wg.Done()
					result, needsReplan := m.executeTask(ctx, out, o, task)
					completed <- completedTask{ID: task.ID, Result: result, Replan: needsReplan}
				}(t)
			}

			if activeTasks == 0 {
				if o.IsComplete() {
					break
				} else {
					out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", Content: "Deadlock detected in execution graph."}
					break
				}
			}

			select {
			case <-ctx.Done():
				return
			case done := <-completed:
				o.MarkComplete(done.ID, done.Result)
				activeTasks--

				// Handle dynamic replanning or subgraph expansion
				if done.Replan {
					newG, err := m.Plan(ctx, "Pivot: "+done.Result)
					if err == nil {
						o.AddTasks(newG.Tasks...)
					}
				} else if strings.Contains(done.Result, "\"tasks\":") {
					re := regexp.MustCompile("(?s)\\{.*\\}")
					match := re.FindString(done.Result)
					if match != "" {
						var subGraph ExecutionGraph
						if err := json.Unmarshal([]byte(match), &subGraph); err == nil {
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

func (m *defaultManager) executeTask(ctx context.Context, out chan<- ChatEvent, o *Orchestrator, task Task) (string, bool) {
	targetAgent, ok := m.agents[task.Agent]
	if !ok {
		out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", TaskID: task.ID, Content: "Agent not found: " + task.Agent}
		return "Agent not found", false
	}

	// Use the persistent session service and the manager's current session ID
	// this ensures that the agent has access to the conversation history.
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

	promptStr := task.Prompt
	if len(contextParts) > 0 {
		promptStr = promptStr + "\n\n### CONTEXT FROM PREVIOUS TASKS\n" + strings.Join(contextParts, "\n\n")
	}

	// We use the same sessionID for all agents in the swarm so they share context
	events := taskRunner.Run(taskCtx, m.userID, m.sessionID, genai.NewContentFromText(promptStr, genai.Role("user")), agent.RunConfig{})

	var full strings.Builder
	var needsReplan bool
	activeToolSpans := make(map[string]Task)

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
					activeToolSpans[part.FunctionCall.Name] = toolTask
				}
				if part.FunctionResponse != nil {
					out <- ChatEvent{Type: ChatEventToolResult, Agent: targetAgent.Name(), TaskID: task.ID, Content: part.FunctionResponse.Name}
					
					// Record tool span completion
					if t, ok := activeToolSpans[part.FunctionResponse.Name]; ok {
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
						delete(activeToolSpans, part.FunctionResponse.Name)
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

	if finalText == "" {
		return "No response emitted by agent.", false
	}

	// Update the global state of the last agent to respond
	m.lastAgent = targetAgent.Name()

	// Output Sanity Check (Output Agent)
	if m.runOutputAgent(ctx, out, o, targetAgent.Name(), finalText) {
		out <- ChatEvent{Type: ChatEventFinalResponse, Agent: targetAgent.Name(), TaskID: task.ID, Content: finalText}
		return finalText, needsReplan
	}

	out <- ChatEvent{Type: ChatEventThought, Agent: "Orchestrator", TaskID: task.ID, Content: "Output Agent rejected the response as problematic."}
	return "Output Agent rejected response.", true
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
