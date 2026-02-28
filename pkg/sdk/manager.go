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

	// Chat sends a natural language prompt to the internal Router Agent.
	// It returns a channel that streams the response back to the caller.
	Chat(ctx context.Context, prompt string) (<-chan string, error)
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

// defaultManager is the internal implementation of AgentManager.
type defaultManager struct {
	run           *runner.Runner
	sessionSvc    session.Service
	userID        string
	sessionID     string
	skills        []*Skill
	clientCfg     *genai.ClientConfig
	pinnedContext map[string]string

	flashModel   model.LLM
	proModel     model.LLM
	toolRegistry map[string]tool.Tool
}

// ManagerConfig defines configuration for the AgentManager.
type ManagerConfig struct {
	// Model overrides the default Gemini model. Useful for testing.
	Model model.LLM
	// ResumeLastSession instructs the manager to load the most recently updated session.
	ResumeLastSession bool
}

// NewManager creates a new instance of the core SDK AgentManager.
func NewManager(cfg ...ManagerConfig) AgentManager {
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
			log.Fatalf("Failed to create flash model: %v", err)
		}
		proModel, err = gemini.NewModel(ctx, proModelName, clientConfig)
		if err != nil {
			log.Fatalf("Failed to create pro model: %v", err)
		}
	}

	// 1. Initialize the global tool registry
	listTool, err := functiontool.New(functiontool.Config{
		Name:        "list_local_files",
		Description: "Lists files in the local directory to help understand the current workspace.",
	}, listLocalFiles)
	if err != nil {
		log.Fatalf("Failed to create listTool: %v", err)
	}

	readTool, err := functiontool.New(functiontool.Config{
		Name:        "read_local_file",
		Description: "Reads the contents of a local file.",
	}, readLocalFile)
	if err != nil {
		log.Fatalf("Failed to create readTool: %v", err)
	}

	grepTool, err := functiontool.New(functiontool.Config{
		Name:        "grep_search",
		Description: "Searches for a regex pattern within files in a directory.",
	}, grepSearch)
	if err != nil {
		log.Fatalf("Failed to create grepTool: %v", err)
	}

	writeTool, err := functiontool.New(functiontool.Config{
		Name:        "write_local_file",
		Description: "Writes content to a file at the specified path. Creates directories if necessary.",
	}, writeLocalFile)
	if err != nil {
		log.Fatalf("Failed to create writeTool: %v", err)
	}

	gitCommit, err := functiontool.New(functiontool.Config{
		Name:        "git_commit",
		Description: "Commits the current directory changes to the local Git repository.",
	}, gitCommitTool)
	if err != nil {
		log.Fatalf("Failed to create gitCommit tool: %v", err)
	}

	gitPush, err := functiontool.New(functiontool.Config{
		Name:        "git_push",
		Description: "Pushes local commits to the remote Git repository.",
	}, gitPushTool)
	if err != nil {
		log.Fatalf("Failed to create gitPush tool: %v", err)
	}

	bashExecute, err := functiontool.New(functiontool.Config{
		Name:        "bash_execute",
		Description: "Executes a shell command using bash. Useful for installing dependencies or testing code.",
	}, bashExecuteTool)
	if err != nil {
		log.Fatalf("Failed to create bashExecute tool: %v", err)
	}

	// A map to resolve string names from tools.yaml to actual ADK Tool instances
	webFetchTool, err := functiontool.New(functiontool.Config{
		Name:        "web_fetch",
		Description: "Fetches and returns the raw text content of a given HTTP/HTTPS URL.",
	}, webFetch)
	if err != nil {
		log.Fatalf("Failed to create webFetch tool: %v", err)
	}

	googleSearchTool, err := functiontool.New(functiontool.Config{
		Name:        "google_search",
		Description: "Performs a Google Search to find up-to-date information on the internet. Provide a query string.",
	}, googleSearchFunc)
	if err != nil {
		log.Fatalf("Failed to create googleSearch tool: %v", err)
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
	}

	// Use persistent SQLite database for sessions
	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".config", "agents")
	os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, "sessions.db")

	sessionSvc, err := database.NewSessionService(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to initialize database session service: %v", err)
	}

	// Ensure the database schema is up-to-date
	if err := database.AutoMigrate(sessionSvc); err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}

	sessionID := ""
	if len(cfg) > 0 && cfg[0].ResumeLastSession {
		resp, err := sessionSvc.List(ctx, &session.ListRequest{
			AppName: "agents-cli",
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
		AppName:   "agents-cli",
		UserID:    "local_user",
		SessionID: sessionID,
	})

	m := &defaultManager{
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
		log.Fatalf("Failed to load agents and skills: %v", err)
	}

	return m
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
	routerInstruction := fmt.Sprintf("You are the primary Router Agent for the Agents CLI. Help the user build, test, and deploy AI agents. Keep your answers brief, professional, and use markdown formatting. Use the list_local_files, read_local_file, and grep_search tools if you need to investigate the workspace. If file contents are provided in the prompt (e.g., via @filename references), use that information to satisfy the user's request. You MUST transfer control to specialized sub-agents (available: %s) for any substantial technical work, file modifications, complex investigations, web research, or broad refactoring.\n\nCRITICAL ROUTING RULES: If you delegate to a sub-agent (like a third-party CLI wrapper) and it returns an error stating the tool is unavailable, not installed, or lacks permissions, DO NOT attempt to route to that specific agent again for the current request. Instead, use your own internal tools or route to a different, capable sub-agent to fulfill the request as a fallback. Maintain this short-term memory of unavailable agents to avoid infinite loops.", strings.Join(subAgentNames, ", "))

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
		AppName:        "agents-cli",
		Agent:          routerAgent,
		SessionService: m.sessionSvc,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %v", err)
	}

	m.run = r
	m.skills = loadedSkills
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
		AppName: "agents-cli",
		UserID:  m.userID,
	})
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".config", "agents", "sessions.db")
	db, dbErr := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	var infos []SessionInfo
	for _, s := range resp.Sessions {
		summary := s.ID()
		if dbErr == nil {
			var event struct {
				Content string
			}
			err := db.Table("events").
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

func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan string, error) {
	out := make(chan string)

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

		if os.Getenv("AGENTS_DRY_RUN") == "true" {
			// Provide fast, deterministic mock responses for vhs tape recordings
			if strings.Contains(strings.ToLower(prompt), "build") {
				out <- "[AGENT_HANDOFF] builder_agent"
				out <- "[builder_agent] I have scaffolded a Python ADK agent for you. I created `agent.yaml`, `requirements.txt`, and `agent.py`."
				return
			}
			if strings.Contains(strings.ToLower(prompt), "test") {
				out <- "[TOOL_CALL] bash_execute"
				out <- "[router_agent] I successfully executed `pip install -r requirements.txt` and `python agent.py` using my bash tool. All tests passed!"
				return
			}
			if strings.Contains(strings.ToLower(prompt), "deploy") {
				out <- "[AGENT_HANDOFF] gitops_agent"
				out <- "[gitops_agent] I have generated `.github/workflows/deploy-agent-engine.yml` and pushed it to `main`. Your agent is deploying to Google Agent Engine."
				return
			}
			out <- "[router_agent] This is a deterministic dry-run response."
			return
		}

		events := m.run.Run(ctx, m.userID, m.sessionID, genai.NewContentFromText(prompt, genai.Role("user")), agent.RunConfig{})

		for event, err := range events {
			if err != nil {
				out <- fmt.Sprintf("Error: %v", err)
				return
			}

			if event.Actions.TransferToAgent != "" {
				out <- fmt.Sprintf("[AGENT_HANDOFF] %s", event.Actions.TransferToAgent)
			}

			if event.Content != nil {
				for _, part := range event.Content.Parts {
					if part.FunctionCall != nil {
						argsStr := ""
						if part.FunctionCall.Args != nil {
							b, err := json.Marshal(part.FunctionCall.Args)
							if err == nil {
								argsStr = " " + string(b)
							}
						}
						out <- fmt.Sprintf("[TOOL_CALL] %s%s", part.FunctionCall.Name, argsStr)
					}
					if part.FunctionResponse != nil {
						respStr := ""
						if part.FunctionResponse.Response != nil {
							b, err := json.Marshal(part.FunctionResponse.Response)
							if err == nil {
								// truncate response so it doesn't flood the UI
								respStr = string(b)
								if len(respStr) > 200 {
									respStr = respStr[:200] + "..."
								}
								respStr = " " + respStr
							}
						}
						out <- fmt.Sprintf("[TOOL_RESULT] %s%s", part.FunctionResponse.Name, respStr)
					}
					// Only stream thoughts if they exist. Wait until final response for actual text.
					if part.Thought {
						// Stream the thought snippet. Truncate it if it's too long.
						thought := part.Text
						if len(thought) > 150 {
							thought = thought[:150] + "..."
						}
						// Replace newlines with spaces for a single line log
						thought = strings.ReplaceAll(thought, "\n", " ")
						out <- fmt.Sprintf("[THOUGHT] %s", thought)
					}
				}
			}

			if !event.Partial && event.IsFinalResponse() {
				var fullResponse strings.Builder

				// Prefix with the author name if it's not the default router
				author := event.Author
				if author == "" {
					author = "agent"
				}
				fullResponse.WriteString(fmt.Sprintf("[%s] ", author))

				if event.Content != nil {
					for _, part := range event.Content.Parts {
						if part.Text != "" && !part.Thought {
							fullResponse.WriteString(part.Text)
						}
					}
				}
				out <- fullResponse.String()
			}
		}
	}()

	return out, nil
}

func (m *defaultManager) Rewind(n int) error {
	if n <= 0 {
		return nil
	}

	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".config", "agents", "sessions.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Find the Nth most recent user event for this session
	var events []struct {
		Timestamp string
	}

	err = db.Table("events").
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
		return db.Table("events").Where("session_id = ?", m.sessionID).Delete(nil).Error
	}

	// Delete all events >= the timestamp of the Nth user event
	targetTime := events[len(events)-1].Timestamp
	return db.Table("events").Where("session_id = ? AND timestamp >= ?", m.sessionID, targetTime).Delete(nil).Error
}
