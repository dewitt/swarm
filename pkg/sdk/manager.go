package sdk

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
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
type AgentManager interface {
	// Discover checks the current directory for an agent.yaml manifest.
	Discover(ctx context.Context, dir string) (*AgentManifest, error)

	// Chat sends a natural language prompt to the internal Router Agent.
	// It returns a channel that streams the response back to the caller.
	Chat(ctx context.Context, prompt string) (<-chan string, error)
	// Reset drops the current conversation history by generating a new session ID.
	Reset()
	// Skills returns a list of all dynamically loaded skills in the workspace.
	Skills() []*Skill
	// ListModels returns a list of available AI models from the provider.
	ListModels(ctx context.Context) ([]ModelInfo, error)
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
	run       *runner.Runner
	userID    string
	sessionID string
	skills    []*Skill
	clientCfg *genai.ClientConfig
}

// ManagerConfig defines configuration for the AgentManager.
type ManagerConfig struct {
	// Model overrides the default Gemini model. Useful for testing.
	Model model.LLM
}

// NewManager creates a new instance of the core SDK AgentManager.
func NewManager(cfg ...ManagerConfig) AgentManager {
	ctx := context.Background()

	var m model.LLM
	clientConfig := &genai.ClientConfig{}
	
	if len(cfg) > 0 && cfg[0].Model != nil {
		m = cfg[0].Model
	} else {
		apiKey := os.Getenv("GOOGLE_API_KEY")

		// We create the model using the API key. If it's empty, the client might fail later
		// but we allow it to initialize so the UI can launch.
		if apiKey != "" {
			clientConfig.APIKey = apiKey
		}

		// Load the user's global config to determine which model to use
		userCfg, _ := LoadConfig() // Ignore error, it falls back to flash safely
		modelName := "gemini-3.1-pro-preview"
		if userCfg != nil && userCfg.Model != "" && userCfg.Model != "auto" {
			modelName = userCfg.Model
		}

		var err error
		m, err = gemini.NewModel(ctx, modelName, clientConfig)
		if err != nil {
			log.Fatalf("Failed to create model: %v", err)
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
	toolRegistry := map[string]tool.Tool{
		"list_local_files": listTool,
		"read_local_file":  readTool,
		"write_local_file": writeTool,
		"git_commit":       gitCommit,
		"git_push":         gitPush,
		"bash_execute":     bashExecute,
	}

	// 2. Dynamically load skills to create sub-agents
	var subAgents []agent.Agent
	var loadedSkills []*Skill

	// Assuming the binary is run from the project root for now. 
	// In a real installation, we would search ~/.config/agents/skills or an embedded FS.
	skillDirs := []string{"skills/builder", "skills/gitops", "skills/adk-skill"}

	for _, dir := range skillDirs {
		skill, err := LoadSkill(dir)
		if err != nil {
			// If we are in tests or running from a weird directory, just skip loading the skill
			// instead of fatally crashing, to preserve development flow.
			continue
		}
		loadedSkills = append(loadedSkills, skill)

		var skillTools []tool.Tool
		for _, toolName := range skill.Manifest.Tools {
			if t, ok := toolRegistry[toolName]; ok {
				skillTools = append(skillTools, t)
			} else {
				log.Printf("Warning: Skill %s requested unknown tool %s", skill.Manifest.Name, toolName)
			}
		}

		skillAgent, err := llmagent.New(llmagent.Config{
			Name:        skill.Manifest.Name,
			Model:       m,
			Description: skill.Manifest.Description,
			Instruction: skill.Instructions,
			Tools:       skillTools,
		})
		if err != nil {
			log.Fatalf("Failed to create agent for skill %s: %v", skill.Manifest.Name, err)
		}
		subAgents = append(subAgents, skillAgent)
	}

	// 3. Create the Router Agent
	routerInstruction := "You are the primary Router Agent for the Agents CLI. Help the user build, test, and deploy AI agents. Keep your answers brief, professional, and use markdown formatting. Use the list_local_files and read_local_file tools if you need to investigate the workspace. If file contents are provided in the prompt (e.g., via @filename references), use that information to satisfy the user's request. Transfer to sub-agents (like builder_agent or gitops_agent) based on the user's intent."

	// Load global memory
	if memory, err := LoadMemory(); err == nil && memory != "" {
		routerInstruction += "\n\nUser Global Preferences & Memory:\n" + memory
	}

	// Look for local instruction files in the current directory
	for _, name := range []string{"GEMINI.md", "AGENTS.md", "CLAUDE.md"} {
		if b, err := os.ReadFile(name); err == nil {
			routerInstruction += "\n\n" + fmt.Sprintf("Additional instructions from %s:\n%s", name, string(b))
		}
	}

	routerAgent, err := llmagent.New(llmagent.Config{
		Name:        "router_agent",
		Model:       m,
		Instruction: routerInstruction,
		Tools:       []tool.Tool{listTool, readTool},
		SubAgents:   subAgents,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	sessionSvc := session.InMemoryService()
	_, err = sessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   "agents-cli",
		UserID:    "local_user",
		SessionID: "local_session",
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        "agents-cli",
		Agent:          routerAgent,
		SessionService: sessionSvc,
	})
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	return &defaultManager{
		run:       r,
		userID:    "local_user",
		sessionID: "local_session",
		skills:    loadedSkills,
		clientCfg: clientConfig,
	}
}

func (m *defaultManager) Skills() []*Skill {
	return m.skills
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
func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan string, error) {
	out := make(chan string)

	go func() {
		defer close(out)

		// Context referencing (@file)
		re := regexp.MustCompile(`@(\S+)`)
		matches := re.FindAllStringSubmatch(prompt, -1)
		if len(matches) > 0 {
			var contextDocs []string
			for _, match := range matches {
				path := match[1]
				// Trim trailing punctuation if it's there (common in conversational text)
				path = strings.TrimRight(path, ".,!?;")
				if b, err := os.ReadFile(path); err == nil {
					contextDocs = append(contextDocs, fmt.Sprintf("File @%s:\n%s", path, string(b)))
				}
			}
			if len(contextDocs) > 0 {
				prompt = strings.Join(contextDocs, "\n\n") + "\n\nUser Prompt:\n" + prompt
			}
		}

		if os.Getenv("AGENTS_DRY_RUN") == "true" {
			// Provide fast, deterministic mock responses for vhs tape recordings
			if strings.Contains(strings.ToLower(prompt), "build") {
				out <- "I have scaffolded a Python ADK agent for you. I created `agent.yaml`, `requirements.txt`, and `agent.py`."
				return
			}
			if strings.Contains(strings.ToLower(prompt), "test") {
				out <- "I successfully executed `pip install -r requirements.txt` and `python agent.py` using my bash tool. All tests passed!"
				return
			}
			if strings.Contains(strings.ToLower(prompt), "deploy") {
				out <- "I have generated `.github/workflows/deploy-agent-engine.yml` and pushed it to `main`. Your agent is deploying to Google Agent Engine."
				return
			}
			out <- "This is a deterministic dry-run response."
			return
		}
		
		events := m.run.Run(ctx, m.userID, m.sessionID, genai.NewContentFromText(prompt, genai.Role("user")), agent.RunConfig{})
		
		for event, err := range events {
			if err != nil {
				out <- fmt.Sprintf("Error: %v", err)
				return
			}
			
			// If it's a partial event, ignore it for now since the CLI waits for the final chunk.
			// Once we implement true streaming in the CLI, we can send partial chunks.
			if !event.Partial && event.IsFinalResponse() {
				var fullResponse strings.Builder
				if event.Content != nil {
					for _, part := range event.Content.Parts {
						if part.Text != "" {
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
