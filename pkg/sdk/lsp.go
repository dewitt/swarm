package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// ManagedLSP abstracts the lifecycle and communication with an MCP-based Language Server.
type ManagedLSP struct {
	mu        sync.RWMutex
	client    *client.Client
	command   string
	args      []string
	connected bool
}

// NewManagedLSP creates a new manager for an LSP running via MCP over stdio.
// Note: This relies on the MCP server wrapper (e.g. mcp-gopls) being available in the path.
func NewManagedLSP(command string, args ...string) *ManagedLSP {
	return &ManagedLSP{
		command: command,
		args:    args,
	}
}

// Start launches the language server subprocess and completes the MCP initialization handshake.
func (l *ManagedLSP) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.connected {
		return nil
	}

	// Create a new stdio client. This automatically manages the subprocess and stdio pipes.
	mcpClient, err := client.NewStdioMCPClient(l.command, nil, l.args...)
	if err != nil {
		return fmt.Errorf("failed to start MCP language server '%s': %w", l.command, err)
	}

	// Initialize the MCP connection
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "swarm-cli",
		Version: "1.0.0",
	}

	// Perform the handshake with a timeout
	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err = mcpClient.Initialize(initCtx, initRequest)
	if err != nil {
		mcpClient.Close()
		return fmt.Errorf("MCP handshake failed: %w", err)
	}

	l.client = mcpClient
	l.connected = true

	return nil
}

// Close gracefully shuts down the language server.
func (l *ManagedLSP) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.connected || l.client == nil {
		return nil
	}

	err := l.client.Close()
	l.connected = false
	l.client = nil
	return err
}

// ListTools retrieves the list of tools provided by the MCP server.
func (l *ManagedLSP) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	l.mu.RLock()
	if !l.connected || l.client == nil {
		l.mu.RUnlock()
		return nil, fmt.Errorf("MCP client is not connected")
	}
	mcpClient := l.client
	l.mu.RUnlock()

	req := mcp.ListToolsRequest{}
	resp, err := mcpClient.ListTools(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Tools, nil
}

// CallTool invokes a specific abstracted tool on the language server.
func (l *ManagedLSP) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	l.mu.RLock()
	if !l.connected || l.client == nil {
		l.mu.RUnlock()
		return nil, fmt.Errorf("MCP client is not connected")
	}
	mcpClient := l.client
	l.mu.RUnlock()

	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	return mcpClient.CallTool(ctx, req)
}
