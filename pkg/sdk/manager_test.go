package sdk_test

import (
	"context"
	"iter"
	"testing"

	"github.com/dewitt/swarm/pkg/sdk"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// MockModel implements model.LLM for testing
type MockModel struct {
	response string
}

func (m *MockModel) Name() string {
	return "mock-model"
}

func (m *MockModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		responseText := m.response
		
		// If system instruction mentions Architect, return a plan
		if req.Config != nil && req.Config.SystemInstruction != nil {
			for _, p := range req.Config.SystemInstruction.Parts {
				if p.Text != "" && (contains(p.Text, "Architect") || contains(p.Text, "DAG")) {
					responseText = `{"tasks": [{"id": "t1", "name": "Test Task", "agent": "router_agent", "prompt": "do it", "dependencies": []}]}`
					break
				}
			}
		}

		resp := &model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{
					{Text: responseText},
				},
				Role: "model",
			},
			TurnComplete: true,
		}
		yield(resp, nil)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewManager(t *testing.T) {
	mockLLM := &MockModel{response: "Hello from the mock agent!"}
	manager, err := sdk.NewManager(sdk.ManagerConfig{Model: mockLLM})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if manager == nil {
		t.Fatal("expected manager to be non-nil")
	}

	ctx := context.Background()

	ch, err := manager.Chat(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var finalResponses []string
	for event := range ch {
		if event.Type == sdk.ChatEventFinalResponse {
			finalResponses = append(finalResponses, event.Content)
		}
	}

	if len(finalResponses) == 0 {
		t.Fatalf("expected at least one final response")
	}

	expected := "Hello from the mock agent!"
	found := false
	for _, r := range finalResponses {
		if r == expected {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected content '%s' in responses, got %v", expected, finalResponses)
	}
}
