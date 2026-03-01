package sdk_test

import (
	"context"
	"iter"
	"strings"
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

		// If system instruction mentions Planning Agent, return a plan
		if req.Config != nil && req.Config.SystemInstruction != nil {
			for _, p := range req.Config.SystemInstruction.Parts {
				if p.Text != "" && (contains(p.Text, "Swarm Agent") || contains(p.Text, "Planning Agent") || contains(p.Text, "DAG")) {
					userPrompt := strings.ToLower(req.Contents[0].Parts[0].Text)
					if contains(userPrompt, "hello") {
						responseText = `{"immediate_response": "Hello from trivial plan!"}`
					} else {
						responseText = `{"spans": [{"id": "t1", "name": "Test Span", "agent": "swarm_agent", "prompt": "do it", "dependencies": []}]}`
					}
					break
				}
				if p.Text != "" && (contains(p.Text, "Input Agent")) {
					responseText = "ROUTE TO: swarm_agent"
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

func TestNewSwarm(t *testing.T) {
	mockLLM := &MockModel{response: "Hello from the mock agent!"}
	swarm, err := sdk.NewSwarm(sdk.SwarmConfig{Model: mockLLM})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if swarm == nil {
		t.Fatal("expected swarm to be non-nil")
	}

	ctx := context.Background()

	ch, err := swarm.Chat(ctx, "hello")
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

	expected := "Hello from trivial plan!"
	found := false
	for _, r := range finalResponses {
		if contains(r, expected) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected content '%s' in responses, got %v", expected, finalResponses)
	}
}

func TestChat_TrivialResponse(t *testing.T) {
	mockLLM := &MockModel{response: "Trivial Response"}
	swarm, err := sdk.NewSwarm(sdk.SwarmConfig{Model: mockLLM})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ctx := context.Background()
	// Trivial prompt that should trigger ImmediateResponse in our MockModel's Planning mode
	ch, err := swarm.Chat(ctx, "Hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var events []sdk.ChatEvent
	for event := range ch {
		events = append(events, event)
	}

	foundFinal := false
	for _, e := range events {
		if e.Type == sdk.ChatEventFinalResponse {
			foundFinal = true
			if e.Content == "" {
				t.Error("expected non-empty final response content")
			}
		}
	}

	if !foundFinal {
		t.Fatalf("expected a final response event for trivial query")
	}
}
