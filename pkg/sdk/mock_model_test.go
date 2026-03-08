package sdk

import (
	"context"
	"iter"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// MockModel implements model.LLM for testing
type MockModel struct {
	Response string
}

func (m *MockModel) Name() string {
	return "mock-model"
}

func (m *MockModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		responseText := m.Response

		// If system instruction mentions Planning Agent, return a plan
		if req.Config != nil && req.Config.SystemInstruction != nil {
			for _, p := range req.Config.SystemInstruction.Parts {
				// Match the new skills/planning-agent/SKILL.md content or legacy names
				if p.Text != "" && (contains(p.Text, "planning_agent") || contains(p.Text, "Planning Agent") || contains(p.Text, "DAG") || contains(p.Text, "ExecutionGraph")) {
					userPrompt := strings.ToLower(req.Contents[0].Parts[0].Text)
					if contains(userPrompt, "hello") {
						responseText = `{"immediate_response": "Hello from trivial plan!"}`
					} else {
						responseText = `{"spans": [{"id": "t1", "name": "Test Span", "agent": "swarm_agent", "prompt": "do it", "dependencies": []}]}`
					}
					break
				}
				// Match the new skills/routing-agent/SKILL.md content
				if p.Text != "" && (contains(p.Text, "routing_agent") || contains(p.Text, "Input Agent") || contains(p.Text, "ROUTE TO")) {
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
	return strings.Contains(s, substr)
}
