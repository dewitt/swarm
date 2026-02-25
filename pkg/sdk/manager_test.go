package sdk_test

import (
	"context"
	"iter"
	"testing"

	"github.com/dewitt/agents/pkg/sdk"
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
		resp := &model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{
					{Text: m.response},
				},
				Role: "model",
			},
			TurnComplete: true,
		}
		yield(resp, nil)
	}
}

func TestNewManager(t *testing.T) {
	mockLLM := &MockModel{response: "Hello from the mock agent!"}
	manager := sdk.NewManager(sdk.ManagerConfig{Model: mockLLM})
	
	if manager == nil {
		t.Fatal("expected manager to be non-nil")
	}

	ctx := context.Background()
	
	ch, err := manager.Chat(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := <-ch
	if response != "Hello from the mock agent!" {
		t.Fatalf("expected response 'Hello from the mock agent!', got '%s'", response)
	}
}
