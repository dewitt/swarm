package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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

type ListFilesArgs struct {
	Dir string `json:"dir"`
}

type ListFilesResult struct {
	Files []string `json:"files"`
}

func listLocalFiles(ctx tool.Context, args ListFilesArgs) (ListFilesResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}
	entries, err := os.ReadDir(args.Dir)
	if err != nil {
		return ListFilesResult{}, err
	}
	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	return ListFilesResult{Files: files}, nil
}

type ReadFileArgs struct {
	Path string `json:"path"`
}

type ReadFileResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func readLocalFile(ctx tool.Context, args ReadFileArgs) (ReadFileResult, error) {
	b, err := os.ReadFile(args.Path)
	if err != nil {
		return ReadFileResult{Error: err.Error()}, nil
	}
	return ReadFileResult{Content: string(b)}, nil
}

type GrepSearchArgs struct {
	Pattern string `json:"pattern"`
	Dir     string `json:"dir"` // Optional, defaults to current directory
}

type GrepSearchResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func grepSearch(ctx tool.Context, args GrepSearchArgs) (GrepSearchResult, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	// We use 'rg' (ripgrep) if available, falling back to standard grep if not.
	// We use a bash wrapper to handle the fallback logic.
	script := fmt.Sprintf(`
		if command -v rg >/dev/null 2>&1; then
			rg -n "%s" "%s"
		else
			grep -rn "%s" "%s"
		fi
	`, args.Pattern, dir, args.Pattern, dir)

	cmd := exec.Command("bash", "-c", script)
	out, err := cmd.CombinedOutput()

	// grep returns exit code 1 if no matches found, which isn't a failure for our tool.
	if err != nil && cmd.ProcessState.ExitCode() != 1 {
		return GrepSearchResult{Error: err.Error() + ": " + string(out)}, nil
	}

	if len(out) == 0 {
		return GrepSearchResult{Output: "No matches found."}, nil
	}

	// Truncate output to prevent massive context bloat
	strOut := string(out)
	if len(strOut) > 10000 {
		strOut = strOut[:10000] + "\n...[TRUNCATED: output too large]..."
	}

	return GrepSearchResult{Output: strOut}, nil
}

type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteFileResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func writeLocalFile(ctx tool.Context, args WriteFileArgs) (WriteFileResult, error) {
	if os.Getenv("AGENTS_DRY_RUN") == "true" {
		return WriteFileResult{Success: true, Error: fmt.Sprintf("[DRY RUN] Would have written %d bytes to %s", len(args.Content), args.Path)}, nil
	}

	// Ensure directory exists
	if dir := filepath.Dir(args.Path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return WriteFileResult{Success: false, Error: err.Error()}, nil
		}
	}
	err := os.WriteFile(args.Path, []byte(args.Content), 0644)
	if err != nil {
		return WriteFileResult{Success: false, Error: err.Error()}, nil
	}
	return WriteFileResult{Success: true}, nil
}

// AgentManager defines the core capabilities of the embeddable SDK.
// It is responsible for orchestrating interactions with the LLM and
// managing local user-defined agents.
//
// By keeping this in the sdk package, we ensure the business logic
// can be compiled via cgo/wasm and consumed by clients other than our CLI.

type WebFetchArgs struct {
	URL string `json:"url"`
}

type WebFetchResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

type GoogleSearchArgs struct {
	Query string `json:"query"`
}

type GoogleSearchResult struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func googleSearchFunc(ctx tool.Context, args GoogleSearchArgs) (GoogleSearchResult, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return GoogleSearchResult{Error: "GOOGLE_API_KEY is not set"}, nil
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return GoogleSearchResult{Error: err.Error()}, nil
	}

	resp, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash",
		genai.Text(args.Query),
		&genai.GenerateContentConfig{
			Tools: []*genai.Tool{
				{GoogleSearch: &genai.GoogleSearch{}},
			},
		},
	)
	if err != nil {
		return GoogleSearchResult{Error: err.Error()}, nil
	}

	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		return GoogleSearchResult{Response: resp.Candidates[0].Content.Parts[0].Text}, nil
	}

	return GoogleSearchResult{Error: "No response from search"}, nil
}

func webFetch(ctx tool.Context, args WebFetchArgs) (WebFetchResult, error) {
	resp, err := http.Get(args.URL)
	if err != nil {
		return WebFetchResult{Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WebFetchResult{Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)}, nil
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebFetchResult{Error: err.Error()}, nil
	}

	content := string(b)
	if len(content) > 20000 {
		content = content[:20000] + "\n...[TRUNCATED: output too large]..."
	}

	return WebFetchResult{Content: content}, nil
}

type SessionInfo struct {
	ID        string
	UpdatedAt string
	Summary   string
}

type AgentManager interface {
	// Discover checks the current directory for an agent.yaml manifest.
	Discover(ctx context.Context, dir string) (*AgentManifest, error)

	// Context Management
	AddContext(path string) error
	DropContext(path string)
	ListContext() []string

	// Plan uses the Architect to decompose a complex prompt into an ExecutionGraph.
	Plan(ctx context.Context, prompt string) (*ExecutionGraph, error)
	// ExecuteGraph executes an ExecutionGraph in parallel and streams events.
	ExecuteGraph(ctx context.Context, g *ExecutionGraph) (<-chan ChatEvent, error)

	// Chat sends a natural language prompt to the internal Router Agent.
	// It returns a channel that streams structured events back to the caller.
	Chat(ctx context.Context, prompt string) (<-chan ChatEvent, error)
	// Reset drops the current conversation history by generating a new session ID.
	Reset()
	// Reload dynamically reloads skills, configuration, and agent prompts without losing session state.
	Reload() error
	// Rewind removes the last n turns from the conversation history.
	Rewind(n int) error
	// Skills returns a list of all dynamically loaded skills in the workspace.
	Skills() []*Skill
	// ListModels returns a list of available AI models from the provider.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// ListSessions returns metadata about the persisted chat sessions.
	ListSessions(ctx context.Context) ([]SessionInfo, error)
}

// ChatEventType defines the type of event being streamed from Chat.
type ChatEventType string

const (
	ChatEventHandoff      ChatEventType = "handoff"
	ChatEventToolCall     ChatEventType = "tool_call"
	ChatEventToolResult   ChatEventType = "tool_result"
	ChatEventTelemetry     ChatEventType = "telemetry"
	ChatEventThought       ChatEventType = "thought"
	ChatEventObserver      ChatEventType = "observer"
	ChatEventReplan        ChatEventType = "replan"
	ChatEventFinalResponse ChatEventType = "final_response"

	ChatEventError        ChatEventType = "error"
)

// ChatEvent represents a structured event streamed during a Chat session.
type ChatEvent struct {
	Type    ChatEventType
	Agent   string
	Content string
}

// ModelInfo contains metadata about an available AI model.
type ModelInfo struct {
	Name        string
	DisplayName string
	Description string
	Version     string
}

// AgentManifest represents a parsed agent.yaml configuration.
type AgentManifest struct {
	Name       string `yaml:"name"`
	Framework  string `yaml:"framework"`
	Language   string `yaml:"language"`
	Entrypoint string `yaml:"entrypoint"`
}

// telemetryContextKey is used to pass a telemetry channel through the context to tools.
type telemetryContextKey struct{}

// defaultManager is the internal implementation of AgentManager.
type defaultManager struct {
	run            *runner.Runner
	db             *gorm.DB
	sessionSvc     session.Service
	userID         string
	sessionID      string
	skills         []*Skill
	clientCfg      *genai.ClientConfig
	pinnedContext  map[string]string
	ciaInstruction string

	flashModel    model.LLM
	proModel      model.LLM
	toolRegistry  map[string]tool.Tool
	ciaAgent      agent.Agent
	subAgentNames []string
	agents        map[string]agent.Agent // Name to Agent mapping
}

// ManagerConfig defines configuration for the AgentManager.
type ManagerConfig struct {
	// Model overrides the default Gemini model. Useful for testing.
	Model model.LLM
	// ResumeLastSession instructs the manager to load the most recently updated session.
	ResumeLastSession bool
}

// NewManager creates a new instance of the core SDK AgentManager.
func NewManager(cfg ...ManagerConfig) (AgentManager, error) {
	ctx := context.Background()

	var flashModel model.LLM
	var proModel model.LLM
	clientConfig := &genai.ClientConfig{}

	if len(cfg) > 0 && cfg[0].Model != nil {
		flashModel = cfg[0].Model
		proModel = cfg[0].Model
	} else {
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey != "" {
			clientConfig.APIKey = apiKey
		}

		// Load user preferences for the 'pro' model
		userCfg, _ := LoadConfig()
		proModelName := "gemini-3.1-pro-preview"
		if userCfg != nil && userCfg.Model != "" && userCfg.Model != "auto" {
			proModelName = userCfg.Model
		}

		var err error
		flashModel, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create flash model: %w", err)
		}
		proModel, err = gemini.NewModel(ctx, proModelName, clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create pro model: %w", err)
		}
	}

	// 1. Initialize the global tool registry
	listTool, err := functiontool.New(functiontool.Config{
		Name:        "list_local_files",
		Description: "Lists files in the local directory to help understand the current workspace.",
	}, listLocalFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to create listTool: %w", err)
	}

	readTool, err := functiontool.New(functiontool.Config{
		Name:        "read_local_file",
		Description: "Reads the contents of a local file.",
	}, readLocalFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create readTool: %w", err)
	}

	grepTool, err := functiontool.New(functiontool.Config{
		Name:        "grep_search",
		Description: "Searches for a regex pattern within files in a directory.",
	}, grepSearch)
	if err != nil {
		return nil, fmt.Errorf("failed to create grepTool: %w", err)
	}

	writeTool, err := functiontool.New(functiontool.Config{
		Name:        "write_local_file",
		Description: "Writes content to a file at the specified path. Creates directories if necessary.",
	}, writeLocalFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create writeTool: %w", err)
	}

	gitCommit, err := functiontool.New(functiontool.Config{
		Name:        "git_commit",
		Description: "Commits the current directory changes to the local Git repository.",
	}, gitCommitTool)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitCommit tool: %w", err)
	}

	gitPush, err := functiontool.New(functiontool.Config{
		Name:        "git_push",
		Description: "Pushes local commits to the remote Git repository.",
	}, gitPushTool)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitPush tool: %w", err)
	}

	bashExecute, err := functiontool.New(functiontool.Config{
		Name:        "bash_execute",
		Description: "Executes a shell command using bash. Useful for installing dependencies or testing code.",
	}, bashExecuteTool)
	if err != nil {
		return nil, fmt.Errorf("failed to create bashExecute tool: %w", err)
	}

	// A map to resolve string names from tools.yaml to actual ADK Tool instances
	webFetchTool, err := functiontool.New(functiontool.Config{
		Name:        "web_fetch",
		Description: "Fetches and returns the raw text content of a given HTTP/HTTPS URL.",
	}, webFetch)
	if err != nil {
		return nil, fmt.Errorf("failed to create webFetch tool: %w", err)
	}

	googleSearchTool, err := functiontool.New(functiontool.Config{
		Name:        "google_search",
		Description: "Performs a Google Search to find up-to-date information on the internet. Provide a query string.",
	}, googleSearchFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create googleSearch tool: %w", err)
	}

	replanTool, err := functiontool.New(functiontool.Config{
		Name:        "request_replan",
		Description: "Requests a global replan of the execution graph based on new discoveries or blocked tasks. Provide a reason and discoveries.",
	}, requestReplan)
	if err != nil {
		return nil, fmt.Errorf("failed to create replanTool: %w", err)
	}

	toolRegistry := map[string]tool.Tool{
		"list_local_files": listTool,
		"read_local_file":  readTool,
		"grep_search":      grepTool,
		"write_local_file": writeTool,
		"git_commit":       gitCommit,
		"git_push":         gitPush,
		"bash_execute":     bashExecute,
		"web_fetch":        webFetchTool,
		"google_search":    googleSearchTool,
		"request_replan":   replanTool,
	}

	// Use persistent SQLite database for sessions
	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".config", "swarm")
	_ = os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, "sessions.db")

	dialector := sqlite.Open(dbPath)
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sessionSvc, err := database.NewSessionService(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database session service: %w", err)
	}

	// Ensure the database schema is up-to-date
	if err := database.AutoMigrate(sessionSvc); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	sessionID := ""
	if len(cfg) > 0 && cfg[0].ResumeLastSession {
		resp, err := sessionSvc.List(ctx, &session.ListRequest{
			AppName: "swarm-cli",
			UserID:  "local_user",
		})
		if err == nil && len(resp.Sessions) > 0 {
			// Find the most recently updated session
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

	// Create the session record if it doesn't already exist
	_, _ = sessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   "swarm-cli",
		UserID:    "local_user",
		SessionID: sessionID,
	})

	m := &defaultManager{
		db:            db,
		sessionSvc:    sessionSvc,
		userID:        "local_user",
		sessionID:     sessionID,
		clientCfg:     clientConfig,
		pinnedContext: make(map[string]string),
		flashModel:    flashModel,
		proModel:      proModel,
		toolRegistry:  toolRegistry,
	}

	if err := m.Reload(); err != nil {
		return nil, fmt.Errorf("failed to load agents and skills: %w", err)
	}

	return m, nil
}

func (m *defaultManager) Reload() error {
	var subAgents []agent.Agent
	var loadedSkills []*Skill

	skillDirs := []string{}
	entries, err := os.ReadDir("skills")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				skillDirs = append(skillDirs, filepath.Join("skills", entry.Name()))
			}
		}
	} else {
		skillDirs = []string{"skills/builder", "skills/gitops", "skills/adk-skill"}
	}

	for _, dir := range skillDirs {
		skill, err := LoadSkill(dir)
		if err != nil {
			continue
		}
		loadedSkills = append(loadedSkills, skill)

		var skillTools []tool.Tool
		skillTools = append(skillTools, m.toolRegistry["request_replan"])
		for _, toolName := range skill.Manifest.Tools {
			if t, ok := m.toolRegistry[toolName]; ok {
				skillTools = append(skillTools, t)
			} else {
				log.Printf("Warning: Skill %s requested unknown tool %s", skill.Manifest.Name, toolName)
			}
		}

		targetModel := m.proModel
		if skill.Manifest.Model == "flash" {
			targetModel = m.flashModel
		}

		skillAgent, err := llmagent.New(llmagent.Config{
			Name:        skill.Manifest.Name,
			Model:       targetModel,
			Description: skill.Manifest.Description,
			Instruction: skill.Instructions,
			Tools:       skillTools,
		})
		if err != nil {
			return fmt.Errorf("failed to create agent for skill %s: %v", skill.Manifest.Name, err)
		}
		subAgents = append(subAgents, skillAgent)
	}

	var subAgentNames []string
	for _, sa := range subAgents {
		subAgentNames = append(subAgentNames, sa.Name())
	}
	routerInstruction := fmt.Sprintf("You are the primary Router Agent for the Swarm CLI. Help the user build, test, and deploy AI agents. Keep your answers brief, professional, and use markdown formatting. Use the list_local_files, read_local_file, and grep_search tools if you need to investigate the workspace. If file contents are provided in the prompt (e.g., via @filename references), use that information to satisfy the user's request. You MUST transfer control to specialized sub-agents (available: %s) for any substantial technical work, file modifications, complex investigations, web research, or broad refactoring.\n\nCRITICAL ROUTING RULES: If you delegate to a sub-agent (like a third-party CLI wrapper) and it returns an error stating the tool is unavailable, not installed, or lacks permissions, DO NOT attempt to route to that specific agent again for the current request. Instead, use your own internal tools or route to a different, capable sub-agent to fulfill the request as a fallback. Maintain this short-term memory of unavailable agents to avoid infinite loops.", strings.Join(subAgentNames, ", "))

	if memory, err := LoadMemory(); err == nil && memory != "" {
		routerInstruction += "\n\nUser Global Preferences & Memory:\n" + memory
	}

	for _, name := range []string{"GEMINI.md", "AGENTS.md", "CLAUDE.md"} {
		if b, err := os.ReadFile(name); err == nil {
			routerInstruction += "\n\n" + fmt.Sprintf("Additional instructions from %s:\n%s", name, string(b))
		}
	}

	routerAgent, err := llmagent.New(llmagent.Config{
		Name:        "router_agent",
		Model:       m.flashModel,
		Instruction: routerInstruction,
		Tools:       []tool.Tool{m.toolRegistry["list_local_files"], m.toolRegistry["read_local_file"], m.toolRegistry["grep_search"]},
		SubAgents:   subAgents,
	})
	if err != nil {
		return fmt.Errorf("failed to create router agent: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        "swarm-cli",
		Agent:          routerAgent,
		SessionService: m.sessionSvc,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %v", err)
	}

	ciaInstruction := fmt.Sprintf(`You are the Chat Input Agent (CIA).
Your job is to classify user input and determine if it should be routed to a specialized sub-agent or the primary router.
Available agents: %s

CRITICAL: You MUST ONLY route to the specific agent names listed above. DO NOT invent or hallucinate new agent names.

If the user input is a digression from the current task, or a new general request, output: "ROUTE TO: router_agent"
If the user input is specifically for one of the specialized agents listed above, output: "ROUTE TO: [agent_name]"
Otherwise, output: "CONTINUE"

Keep your analysis silent. ONLY output the routing decision.`, strings.Join(m.subAgentNames, ", "))

	m.run = r
	m.skills = loadedSkills
	m.subAgentNames = subAgentNames
	m.ciaInstruction = ciaInstruction
	
	m.agents = make(map[string]agent.Agent)
	m.agents[routerAgent.Name()] = routerAgent
	for _, sa := range subAgents {
		m.agents[sa.Name()] = sa
	}

	// Initialize the Chat Input Agent (CIA)
	ciaAgent, err := llmagent.New(llmagent.Config{
		Name:        "chat_input_agent",
		Model:       m.flashModel,
		Instruction: ciaInstruction,
	})
	if err != nil {
		return fmt.Errorf("failed to create chat input agent: %v", err)
	}
	m.ciaAgent = ciaAgent

	return nil
}

func (m *defaultManager) Skills() []*Skill {
	return m.skills
}

func (m *defaultManager) AddContext(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}
	m.pinnedContext[path] = string(b)
	return nil
}

func (m *defaultManager) DropContext(path string) {
	if path == "all" {
		m.pinnedContext = make(map[string]string)
	} else {
		delete(m.pinnedContext, path)
	}
}

func (m *defaultManager) ListContext() []string {
	var paths []string
	for path := range m.pinnedContext {
		paths = append(paths, path)
	}
	return paths
}

func (m *defaultManager) Reset() {
	m.sessionID = fmt.Sprintf("session_%d", rand.Int63())
}

func (m *defaultManager) ListModels(ctx context.Context) ([]ModelInfo, error) {
	client, err := genai.NewClient(ctx, m.clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	var models []ModelInfo
	iter := client.Models.All(ctx)
	for modelObj, err := range iter {
		if err != nil {
			return nil, fmt.Errorf("error fetching models: %w", err)
		}

		// Simplify the output by grabbing the clean display name or name
		name := modelObj.Name
		if strings.HasPrefix(name, "models/") {
			name = strings.TrimPrefix(name, "models/")
		}

		models = append(models, ModelInfo{
			Name:        name,
			DisplayName: modelObj.DisplayName,
			Description: modelObj.Description,
			Version:     modelObj.Version,
		})
	}
	return models, nil
}
func (m *defaultManager) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	resp, err := m.sessionSvc.List(ctx, &session.ListRequest{
		AppName: "swarm-cli",
		UserID:  m.userID,
	})
	if err != nil {
		return nil, err
	}

	var infos []SessionInfo
	for _, s := range resp.Sessions {
		summary := s.ID()
		var event struct {
			Content string
		}
		err := m.db.Table("events").
			Select("content").
			Where("session_id = ? AND author = ?", s.ID(), "user").
			Order("timestamp DESC").
			Limit(1).
			Find(&event).Error

		if err == nil && event.Content != "" {
			var c map[string]interface{}
			if json.Unmarshal([]byte(event.Content), &c) == nil {
				if parts, ok := c["parts"].([]interface{}); ok && len(parts) > 0 {
					if part, ok := parts[0].(map[string]interface{}); ok {
						if text, ok := part["text"].(string); ok {
							summary = text
						}
					}
				}
			}
		}

		if len(summary) > 40 {
			summary = summary[:37] + "..."
		}
		summary = strings.ReplaceAll(summary, "\n", " ")

		infos = append(infos, SessionInfo{
			ID:        s.ID(),
			UpdatedAt: s.LastUpdateTime().Format("2006-01-02 15:04:05"),
			Summary:   summary,
		})
	}
	return infos, nil
}

func (m *defaultManager) Plan(ctx context.Context, prompt string) (*ExecutionGraph, error) {
	systemPrompt := fmt.Sprintf(`You are the Swarm Architect.
Your job is to take a complex user request and decompose it into a Directed Acyclic Graph (DAG) of tasks to be executed by specialized agents.
Available agents: %s

TRIVIAL QUERIES: If the user's request is trivial (e.g., "Hello", "How are you?", or simple calculations like "2+2") and does not require specialized agent work, DO NOT generate tasks. Instead, provide the answer in the "immediate_response" field.

You MUST output ONLY valid JSON matching this schema:
{
  "tasks": [
    {
      "id": "unique_string_id",
      "name": "Short name",
      "agent": "agent_name",
      "prompt": "Detailed instructions for the agent",
      "dependencies": ["id_of_another_task"]
    }
  ],
  "immediate_response": "Optional direct answer for trivial queries"
}
Do not wrap the JSON in Markdown code blocks. Just output the raw JSON.`, strings.Join(m.subAgentNames, ", "))

	respIter := m.proModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system")),
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

	jsonStr = strings.TrimSpace(jsonStr)
	// Strip markdown formatting if the model ignored the instruction
	if strings.HasPrefix(jsonStr, "```json") {
		jsonStr = strings.TrimPrefix(jsonStr, "```json")
		jsonStr = strings.TrimSuffix(jsonStr, "```")
		jsonStr = strings.TrimSpace(jsonStr)
	}

	var graph ExecutionGraph
	if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil {
		return nil, fmt.Errorf("failed to parse DAG JSON: %w\nResponse was: %s", err, jsonStr)
	}

	return &graph, nil
}

func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan ChatEvent, error) {
	out := make(chan ChatEvent)

	go func() {
		defer close(out)

		// Context referencing (@file)
		re := regexp.MustCompile(`@(\S+)`)
		matches := re.FindAllStringSubmatch(prompt, -1)

		var contextDocs []string

		if len(matches) > 0 {
			for _, match := range matches {
				path := match[1]
				// Trim trailing punctuation if it's there (common in conversational text)
				path = strings.TrimRight(path, ".,!?;")
				if b, err := os.ReadFile(path); err == nil {
					contextDocs = append(contextDocs, fmt.Sprintf("File @%s:\n%s", path, string(b)))
				}
			}
		}

		// Inject pinned Context Management files
		for path, content := range m.pinnedContext {
			contextDocs = append(contextDocs, fmt.Sprintf("Pinned File %s:\n%s", path, content))
		}

		if len(contextDocs) > 0 {
			prompt = strings.Join(contextDocs, "\n\n") + "\n\nUser Prompt:\n" + prompt
		}

		// --- 1. Chat Input Agent (CIA) Pre-processing ---
		out <- ChatEvent{Type: ChatEventThought, Agent: "CIA", Content: "Classifying intent…"}
		ciaRespIter := m.flashModel.GenerateContent(ctx, &model.LLMRequest{
			Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
			Config: &genai.GenerateContentConfig{
				SystemInstruction: genai.NewContentFromText(m.ciaInstruction, genai.Role("system")),
			},
		}, false)

		for ciaResp, err := range ciaRespIter {
			if err == nil && ciaResp.Content != nil && len(ciaResp.Content.Parts) > 0 {
				ciaText := ciaResp.Content.Parts[0].Text
				if strings.HasPrefix(ciaText, "ROUTE TO: ") {
					target := strings.TrimPrefix(ciaText, "ROUTE TO: ")
					target = strings.TrimSpace(target)
					if target != "" && target != "router_agent" {
						out <- ChatEvent{Type: ChatEventThought, Agent: "CIA", Content: "Rerouting to " + target}
						// Manual handoff for CIA classification
						sessResp, err := m.sessionSvc.Get(ctx, &session.GetRequest{
							AppName:   "swarm-cli",
							UserID:    m.userID,
							SessionID: m.sessionID,
						})
						if err == nil {
							transferEvent := session.NewEvent(fmt.Sprintf("cia_%d", time.Now().UnixNano()))
							transferEvent.Author = "cia"
							transferEvent.Actions.TransferToAgent = target
							_ = m.sessionSvc.AppendEvent(ctx, sessResp.Session, transferEvent)
							out <- ChatEvent{Type: ChatEventHandoff, Content: target}
						}
					}
				}
			}
			break
		}

		// --- 2. Byzantine Swarm Orchestration (Planning) ---
		out <- ChatEvent{Type: ChatEventThought, Agent: "Architect", Content: "Planning swarm execution…"}
		graph, err := m.Plan(ctx, prompt)
		if err != nil {
			out <- ChatEvent{Type: ChatEventError, Agent: "Architect", Content: "Failed to generate execution plan: " + err.Error()}
			return
		}

		// Helper to run COA sanity check
		runCOA := func(content string) {
			coaPrompt := fmt.Sprintf(`You are the Chat Output Agent (COA).
Sanity check the following response from a swarm agent to the user.
Goal: Ensure the response is helpful, accurate, and respects the user's time.

Response:
%s

If the response is clearly wrong, hallucinating, or missing the point, output: "FIX: [Reason and suggestion]".
Otherwise, output: "OK".`, content)

			coaRespIter := m.flashModel.GenerateContent(ctx, &model.LLMRequest{
				Contents: []*genai.Content{genai.NewContentFromText(coaPrompt, genai.Role("user"))},
			}, false)

			for coaResp, err := range coaRespIter {
				if err == nil && coaResp.Content != nil && len(coaResp.Content.Parts) > 0 {
					text := coaResp.Content.Parts[0].Text
					if strings.HasPrefix(text, "FIX:") {
						out <- ChatEvent{Type: ChatEventThought, Agent: "COA", Content: "Sanity check failed: " + strings.TrimPrefix(text, "FIX:")}
					}
				}
				break
			}
		}

		if graph.ImmediateResponse != "" {
			runCOA(graph.ImmediateResponse)
			out <- ChatEvent{Type: ChatEventFinalResponse, Agent: "Architect", Content: graph.ImmediateResponse}
			return
		}

		// --- 3. Execute the Reactive Task Pool ---
		events, err := m.ExecuteGraph(ctx, graph)
		if err != nil {
			out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", Content: "Failed to start swarm: " + err.Error()}
			return
		}

		for event := range events {
			if event.Type == ChatEventFinalResponse {
				runCOA(event.Content)
			}
			out <- event
		}
	}()

	return out, nil
}

func (m *defaultManager) ExecuteGraph(ctx context.Context, g *ExecutionGraph) (<-chan ChatEvent, error) {
	out := make(chan ChatEvent)
	go func() {
		defer close(out)
		o := NewOrchestrator(g)

		type completedTask struct {
			ID     string
			Result string
			Replan bool
		}
		completed := make(chan completedTask)
		activeTasks := 0

		for {
			ready := o.GetReadyTasks()
			for _, t := range ready {
				o.MarkActive(t.ID)
				activeTasks++

				go func(task Task) {
					var taskResult string
					var needsReplan bool
					defer func() {
						completed <- completedTask{ID: task.ID, Result: taskResult, Replan: needsReplan}
					}()

					targetAgent, ok := m.agents[task.Agent]
					if !ok {
						out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", Content: fmt.Sprintf("Agent '%s' not found for task '%s'", task.Agent, task.ID)}
						return
					}

					taskSessionID := fmt.Sprintf("%s_%s", m.sessionID, task.ID)
					_, _ = m.sessionSvc.Create(ctx, &session.CreateRequest{
						AppName:   "swarm-cli",
						UserID:    m.userID,
						SessionID: taskSessionID,
					})

					taskRunner, err := runner.New(runner.Config{
						AppName:        "swarm-cli",
						Agent:          targetAgent,
						SessionService: m.sessionSvc,
					})
					if err != nil {
						out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", Content: fmt.Sprintf("Failed to init runner for task '%s': %v", task.ID, err)}
						return
					}

					out <- ChatEvent{Type: ChatEventThought, Agent: targetAgent.Name(), Content: fmt.Sprintf("Starting task: %s", task.Name)}

					telemetryChan := make(chan string, 100)
					toolCtx, cancelToolCtx := context.WithCancel(context.WithValue(ctx, telemetryContextKey{}, (chan<- string)(telemetryChan)))
					defer cancelToolCtx()

					telemetryDone := make(chan struct{})
					go func() {
						var lastLines []string
						for line := range telemetryChan {
							out <- ChatEvent{Type: ChatEventTelemetry, Agent: targetAgent.Name(), Content: line}
							
							// Buffer telemetry for the Observer
							lastLines = append(lastLines, line)
							if len(lastLines) > 10 {
								lastLines = lastLines[1:]
							}

							// If we've seen a lot of telemetry, trigger an Observer check
							if len(lastLines) >= 10 {
								go func(history []string) {
									obsCtx, obsCancel := context.WithTimeout(ctx, 10*time.Second)
									defer obsCancel()

									obsPrompt := fmt.Sprintf(`You are the Swarm Observer.
Monitor the telemetry of the active agent: %s
Current Task: %s
Recent Telemetry:
%s

Determine if the agent is stuck in a loop, repeating the same tool call with the same parameters, or deviating from the original goal.
If the agent is stuck or deviating, output: "INTERVENE: [Reason]".
Otherwise, output: "OK".`, targetAgent.Name(), task.Name, strings.Join(history, "\n"))

									respIter := m.flashModel.GenerateContent(obsCtx, &model.LLMRequest{
										Contents: []*genai.Content{genai.NewContentFromText(obsPrompt, genai.Role("user"))},
									}, false)

									for resp, err := range respIter {
										if err == nil && resp.Content != nil && len(resp.Content.Parts) > 0 {
											text := resp.Content.Parts[0].Text
											if strings.HasPrefix(text, "INTERVENE:") {
												out <- ChatEvent{Type: ChatEventObserver, Agent: "Observer", Content: fmt.Sprintf("Intervention requested for %s: %s", targetAgent.Name(), strings.TrimPrefix(text, "INTERVENE:"))}
												// Here we could force a context cancel, but for now we just observe and report
											}
										}
										break
									}
								}(lastLines)
								// Reset history to avoid continuous checks
								lastLines = nil
							}
						}
						close(telemetryDone)
					}()

					// Inject results from completed dependencies into the prompt
					depContext := ""
					results := o.GetContext()
					for _, depID := range task.Dependencies {
						if res, ok := results[depID]; ok {
							depContext += fmt.Sprintf("\n--- Result from %s ---\n%s\n", depID, res)
						}
					}

					promptStr := fmt.Sprintf("Task: %s\nInstructions: %s\n%s", task.Name, task.Prompt, depContext)
					events := taskRunner.Run(toolCtx, m.userID, taskSessionID, genai.NewContentFromText(promptStr, genai.Role("user")), agent.RunConfig{})

					var fullResponse strings.Builder
					for event, err := range events {
						if err != nil {
							out <- ChatEvent{Type: ChatEventError, Agent: targetAgent.Name(), Content: err.Error()}
							break
						}
						if event.Content != nil {
							for _, part := range event.Content.Parts {
								if part.FunctionCall != nil {
									if part.FunctionCall.Name == "request_replan" {
										needsReplan = true
									}
									argsStr := ""
									if part.FunctionCall.Args != nil {
										b, err := json.Marshal(part.FunctionCall.Args)
										if err == nil {
											argsStr = " " + string(b)
										}
									}
									out <- ChatEvent{Type: ChatEventToolCall, Agent: targetAgent.Name(), Content: fmt.Sprintf("%s%s", part.FunctionCall.Name, argsStr)}
								}
								if part.FunctionResponse != nil {
									out <- ChatEvent{Type: ChatEventToolResult, Agent: targetAgent.Name(), Content: fmt.Sprintf("%s completed", part.FunctionResponse.Name)}
								}
							}
						}
						if !event.Partial && event.IsFinalResponse() {
							if event.Content != nil {
								for _, part := range event.Content.Parts {
									if part.Text != "" && !part.Thought {
										fullResponse.WriteString(part.Text)
									}
								}
							}
							out <- ChatEvent{Type: ChatEventFinalResponse, Agent: targetAgent.Name(), Content: fullResponse.String()}
						}
					}
					taskResult = fullResponse.String()

					close(telemetryChan)
					<-telemetryDone
				}(t)
			}

			if activeTasks == 0 {
				if o.IsComplete() {
					break
				} else {
					out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", Content: "Deadlock detected: No active tasks and unresolved dependencies."}
					break
				}
			}

			select {
			case <-ctx.Done():
				return
			case done := <-completed:
				o.MarkComplete(done.ID, done.Result)
				activeTasks--

				// --- Reactive Branching & Replanning Logic ---
				if done.Replan {
					out <- ChatEvent{Type: ChatEventReplan, Agent: "Orchestrator", Content: fmt.Sprintf("Task %s requested a replan. Pivoting swarm…", done.ID)}
					// Combine the user's original goal with the reason for replanning
					// (In a real implementation, we'd pull the exact 'reason' arg from the tool call)
					newGraph, err := m.Plan(ctx, fmt.Sprintf("Pivoting plan based on discovery in task %s: %s", done.ID, done.Result))
					if err == nil {
						o.AddTasks(newGraph.Tasks...)
						out <- ChatEvent{Type: ChatEventThought, Agent: "Orchestrator", Content: fmt.Sprintf("Graph mutated with %d new/updated tasks.", len(newGraph.Tasks))}
					} else {
						out <- ChatEvent{Type: ChatEventError, Agent: "Orchestrator", Content: "Replanning failed: " + err.Error()}
					}
				} else if strings.Contains(done.Result, "\"tasks\":") {
					startIdx := strings.Index(done.Result, "{")
					endIdx := strings.LastIndex(done.Result, "}")
					if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
						jsonStr := done.Result[startIdx : endIdx+1]
						var subGraph ExecutionGraph
						if err := json.Unmarshal([]byte(jsonStr), &subGraph); err == nil {
							for i := range subGraph.Tasks {
								subGraph.Tasks[i].ParentID = done.ID
								// Ensure sub-tasks don't clash on IDs
								subGraph.Tasks[i].ID = fmt.Sprintf("%s_%s", done.ID, subGraph.Tasks[i].ID)
							}
							o.AddTasks(subGraph.Tasks...)
							out <- ChatEvent{Type: ChatEventThought, Agent: "Orchestrator", Content: fmt.Sprintf("Task %s spawned %d sub-tasks.", done.ID, len(subGraph.Tasks))}
						}
					}
				}
			}
		}
	}()
	return out, nil
}

func (m *defaultManager) Rewind(n int) error {
	if n <= 0 {
		return nil
	}

	// Find the Nth most recent user event for this session
	var events []struct {
		Timestamp string
	}

	err := m.db.Table("events").
		Select("timestamp").
		Where("session_id = ? AND author = ?", m.sessionID, "user").
		Order("timestamp DESC").
		Limit(n).
		Find(&events).Error

	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if len(events) < n {
		// Not enough user events, just delete all events for this session
		return m.db.Table("events").Where("session_id = ?", m.sessionID).Delete(nil).Error
	}

	// Delete all events >= the timestamp of the Nth user event
	targetTime := events[len(events)-1].Timestamp
	return m.db.Table("events").Where("session_id = ? AND timestamp >= ?", m.sessionID, targetTime).Delete(nil).Error
}
