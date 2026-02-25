package sdk

import "context"

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
	Name      string `yaml:"name"`
	Framework string `yaml:"framework"`
	Language  string `yaml:"language"`
}

// defaultManager is the internal implementation of AgentManager.
type defaultManager struct {
	// TODO: Phase 2 will embed the ADK LlmAgent here.
}

// NewManager creates a new instance of the core SDK AgentManager.
func NewManager() AgentManager {
	return &defaultManager{}
}

func (m *defaultManager) Discover(ctx context.Context, dir string) (*AgentManifest, error) {
	// Stub implementation for Phase 1
	return nil, nil
}

func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan string, error) {
	// Stub implementation for Phase 1
	out := make(chan string)
	go func() {
		defer close(out)
		out <- "Hello from the internal Router Agent SDK stub!"
	}()
	return out, nil
}
