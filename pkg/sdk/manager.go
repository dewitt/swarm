package sdk

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	if len(cfg) > 0 && cfg[0].Model != nil {
		m = cfg[0].Model
	} else {
		apiKey := os.Getenv("GOOGLE_API_KEY")
		
		// We create the model using the API key. If it's empty, the client might fail later
		// but we allow it to initialize so the UI can launch.
		clientConfig := &genai.ClientConfig{}
		if apiKey != "" {
			clientConfig.APIKey = apiKey
		}

		var err error
		m, err = gemini.NewModel(ctx, "gemini-2.5-flash", clientConfig)
		if err != nil {
			log.Fatalf("Failed to create model: %v", err)
		}
	}

	listTool, err := functiontool.New(functiontool.Config{
		Name:        "list_local_files",
		Description: "Lists files in the local directory to help understand the current workspace.",
	}, listLocalFiles)
	if err != nil {
		log.Fatalf("Failed to create listTool: %v", err)
	}

	writeTool, err := functiontool.New(functiontool.Config{
		Name:        "write_local_file",
		Description: "Writes content to a file at the specified path. Creates directories if necessary.",
	}, writeLocalFile)
	if err != nil {
		log.Fatalf("Failed to create writeTool: %v", err)
	}

	builderAgent, err := llmagent.New(llmagent.Config{
		Name:        "builder_agent",
		Model:       m,
		Description: "Specialized in scaffolding new agent projects across different frameworks (ADK, LangGraph) and writing boilerplate code.",
		Instruction: "You are the Builder Agent. Scaffold projects and write initial agent.yaml, requirements.txt, and agent scripts. Use the write_local_file tool to create them.",
		Tools:       []tool.Tool{writeTool},
	})
	if err != nil {
		log.Fatalf("Failed to create builder agent: %v", err)
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

	gitopsAgent, err := llmagent.New(llmagent.Config{
		Name:        "gitops_agent",
		Model:       m,
		Description: "Specialized in crafting CI/CD pipelines, writing GitHub Actions, and executing Git operations for deployment.",
		Instruction: "You are the GitOps Agent. When asked to deploy an agent, scaffold a standard deployment workflow (e.g., GitHub Actions in .github/workflows/), use write_local_file to save it, then use git_commit and git_push to deploy it.",
		Tools:       []tool.Tool{writeTool, gitCommit, gitPush},
	})
	if err != nil {
		log.Fatalf("Failed to create gitops agent: %v", err)
	}

	routerAgent, err := llmagent.New(llmagent.Config{
		Name:        "router_agent",
		Model:       m,
		Instruction: "You are the primary Router Agent for the Agents CLI. Help the user build, test, and deploy AI agents. Keep your answers brief, professional, and use markdown formatting. Use the list_local_files tool if you need to inspect the workspace. If the user wants to build or scaffold a new project, transfer to the builder_agent. If the user wants to deploy the agent to production via CI/CD, transfer to the gitops_agent.",
		Tools:       []tool.Tool{listTool},
		SubAgents:   []agent.Agent{builderAgent, gitopsAgent},
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
	}
}

func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan string, error) {
	out := make(chan string)

	go func() {
		defer close(out)
		
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
