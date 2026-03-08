package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type Reflection struct {
	IsResolved     bool     `json:"is_resolved"`
	NeedsUserInput bool     `json:"needs_user_input"`
	Reasoning      string   `json:"reasoning"`
	NextSteps      string   `json:"next_steps"`
	NewFacts       []string `json:"new_facts,omitempty"`
}

func (m *defaultSwarm) Reflect(ctx context.Context, prompt string, traj Trajectory) (*Reflection, error) {
	b, _ := json.MarshalIndent(traj.Spans, "", "  ")

	systemPrompt := `You are Swarm. Your job is to evaluate whether the user's original goal has been FULLY completed based on the execution trajectory.
If it is completed, set is_resolved to true.
If the agent only diagnosed a problem but did not physically apply the fix (e.g., using write_local_file), then the task is NOT resolved. You must set is_resolved to false and provide explicit next_steps to implement the fix.
If a sub-agent explicitly asked the user a question (e.g., "Do you approve this action?", "Which option do you prefer?"), or if the swarm is completely stuck and requires human intervention to proceed, you MUST set needs_user_input to true and return the question in the reasoning.

Additionally, you MUST identify any "timeless facts" discovered during this execution. A timeless fact is a piece of information that is true across sessions and projects (e.g., "The build command for this repo is X", "The secret ingredient is Y", "Tool Z is currently down").
If you find such facts, include them in the "new_facts" array. Be exhaustive but precise. Avoid ephemeral conversational state.

Output your response as strictly valid JSON matching this schema:
{
  "is_resolved": boolean,
  "needs_user_input": boolean,
  "reasoning": "string",
  "next_steps": "string",
  "new_facts": ["string"]
}`

	userPrompt := fmt.Sprintf("Original Goal: %s\n\nExecution Trajectory:\n%s", prompt, string(b))

	respIter := m.proModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(userPrompt, genai.Role("user"))},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemPrompt, genai.Role("system")),
			ResponseMIMEType:  "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"is_resolved":      {Type: genai.TypeBoolean, Description: "True if the original goal has been fully completed and verified."},
					"needs_user_input": {Type: genai.TypeBoolean, Description: "True if an agent explicitly asked the user a question or needs human intervention."},
					"reasoning":        {Type: genai.TypeString, Description: "Explanation for why the task is or isn't resolved. If needs_user_input is true, state the question here."},
					"next_steps":       {Type: genai.TypeString, Description: "Explicit instructions for what to do next if not resolved."},
					"new_facts": {
						Type:        genai.TypeArray,
						Items:       &genai.Schema{Type: genai.TypeString},
						Description: "Timeless facts discovered during this execution to be saved to semantic memory.",
					},
				},
				Required: []string{"is_resolved", "needs_user_input", "reasoning"},
			},
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

	jsonStr = extractJSON(jsonStr)
	var reflection Reflection
	if err := json.Unmarshal([]byte(jsonStr), &reflection); err != nil {
		return nil, fmt.Errorf("failed to parse reflection from LLM output (expected JSON): %w\nRaw output:\n%s", err, jsonStr)
	}
	return &reflection, nil
}

func (m *defaultSwarm) SummarizeState(ctx context.Context, state string) (string, error) {
	prompt := fmt.Sprintf("You are an observer monitoring a swarm of AI agents. Here is their current activity:\n%s\n\nWrite a single, concise sentence (max 8 words) summarizing their overall progress to the user. Start with an action verb (e.g., 'Investigating the codebase', 'Running tests and debugging').", state)

	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
	}, false)

	for resp, err := range respIter {
		if err != nil {
			return "", err
		}
		if resp.Content != nil && len(resp.Content.Parts) > 0 {
			text := strings.TrimSpace(resp.Content.Parts[0].Text)
			text = strings.TrimPrefix(text, "\"")
			text = strings.TrimSuffix(text, "\"")
			return text, nil
		}
		break
	}
	return "Working...", nil
}
