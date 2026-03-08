package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// extractJSON attempts to locate and extract a JSON object or array from a string
// that may be wrapped in markdown or contain leading/trailing conversational text.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")

	start := -1
	end := -1

	if startObj != -1 && (startArr == -1 || startObj < startArr) {
		start = startObj
		end = strings.LastIndex(s, "}")
	} else if startArr != -1 && (startObj == -1 || startArr < startObj) {
		start = startArr
		end = strings.LastIndex(s, "]")
	}

	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}

	return s
}

func (m *defaultSwarm) Plan(ctx context.Context, prompt string, traj Trajectory) (*ExecutionGraph, error) {
	// Dynamically reload skills from disk so that skills created during this session (e.g. by the skill_builder_agent) are immediately available.
	_ = m.Reload()

	promptForLLM := prompt
	if len(traj.Spans) > 0 {
		b, _ := json.MarshalIndent(traj.Spans, "", "  ")
		promptForLLM = promptForLLM + "\n\n### PREVIOUS EXECUTION TRAJECTORY:\n" + string(b)
	}

	// Recompile the descriptions since they aren't stored on the struct
	m.mu.RLock()
	var descriptions []string
	for _, name := range m.subAgentNames {
		if a, ok := m.agents[name]; ok {
			descriptions = append(descriptions, fmt.Sprintf("- **%s**: %s", name, a.Description()))
		}
	}
	specialistsList := strings.Join(descriptions, "\n")
	routingInstruction := m.routingInstruction
	planningInstruction := m.planningInstruction
	m.mu.RUnlock()

	routingPrompt := fmt.Sprintf(routingInstruction, specialistsList)

	// Inject any critical semantic facts (like tool deprecations) or facts directly relevant to the user's prompt
	if m.memory != nil && m.memory.Semantic() != nil {
		var injectedFacts []string

		// 1. Always look for system-level tool failures
		if sysFacts, err := m.memory.Semantic().Retrieve("TOOL FAILURE OFFLINE", 3); err == nil && len(sysFacts) > 0 {
			injectedFacts = append(injectedFacts, sysFacts...)
		}

		// 2. Look for semantic memories related to the user's actual prompt (not the trajectory json)
		if userFacts, err := m.memory.Semantic().Retrieve(prompt, 3); err == nil && len(userFacts) > 0 {
			injectedFacts = append(injectedFacts, userFacts...)
		}

		if len(injectedFacts) > 0 {
			routingPrompt += "\n\n### CRITICAL SYSTEM FACTS (SEMANTIC MEMORY):\n" + strings.Join(injectedFacts, "\n")
		}
	}
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(promptForLLM, genai.Role("user"))},
		Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(routingPrompt, genai.Role("system"))},
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
	jsonStr = strings.TrimSpace(jsonStr)
	if strings.Contains(jsonStr, "DEEP_PLAN_REQUIRED") {
		planningPrompt := fmt.Sprintf(planningInstruction, specialistsList)

		if m.memory != nil && m.memory.Semantic() != nil {
			var injectedFacts []string
			if facts, err := m.memory.Semantic().Retrieve("TOOL FAILURE OFFLINE", 5); err == nil && len(facts) > 0 {
				injectedFacts = append(injectedFacts, facts...)
			}
			if userFacts, err := m.memory.Semantic().Retrieve(prompt, 5); err == nil && len(userFacts) > 0 {
				injectedFacts = append(injectedFacts, userFacts...)
			}
			if len(injectedFacts) > 0 {
				planningPrompt += "\n\n### RELEVANT SYSTEM FACTS (SEMANTIC MEMORY):\n" + strings.Join(injectedFacts, "\n")
			}
		}

		respIter = m.proModel.GenerateContent(ctx, &model.LLMRequest{
			Contents: []*genai.Content{genai.NewContentFromText(promptForLLM, genai.Role("user"))},
			Config: &genai.GenerateContentConfig{
				SystemInstruction: genai.NewContentFromText(planningPrompt, genai.Role("system")),
				ResponseMIMEType:  "application/json",
				ResponseSchema: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"immediate_response": {Type: genai.TypeString, Description: "Use this to short-circuit the planning phase and respond directly to trivial conversational requests without delegating to sub-agents."},
						"spans": {
							Type: genai.TypeArray,
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"id":             {Type: genai.TypeString, Description: "A unique identifier for this span, like 't1'."},
									"operation_name": {Type: genai.TypeString, Description: "A short, descriptive name for the task."},
									"agent":          {Type: genai.TypeString, Description: "The exact name of the sub-agent to execute this span."},
									"prompt":         {Type: genai.TypeString, Description: "The comprehensive instruction prompt for the sub-agent."},
									"dependencies": {
										Type:        genai.TypeArray,
										Items:       &genai.Schema{Type: genai.TypeString},
										Description: "An array of span IDs that must successfully complete before this span can begin.",
									},
								},
								Required: []string{"id", "operation_name", "agent", "prompt", "dependencies"},
							},
						},
					},
				},
			},
		}, false)
		jsonStr = ""
		for resp, err := range respIter {
			if err != nil {
				return nil, err
			}
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				jsonStr += resp.Content.Parts[0].Text
			}
		}
		jsonStr = strings.TrimSpace(jsonStr)
	}
	jsonStr = strings.TrimSpace(jsonStr)

	// Remove markdown ticks if the LLM ignores the schema constraints
	if strings.HasPrefix(jsonStr, "```json") {
		jsonStr = strings.TrimPrefix(jsonStr, "```json")
		jsonStr = strings.TrimSuffix(jsonStr, "```")
		jsonStr = strings.TrimSpace(jsonStr)
	}

	jsonStr = extractJSON(jsonStr)

	var graph ExecutionGraph

	// If extractJSON couldn't find a JSON object or array, and the string is just conversational text,
	// gracefully wrap it in an immediate response instead of crashing.
	if jsonStr != "" && !strings.HasPrefix(jsonStr, "{") && !strings.HasPrefix(jsonStr, "[") {
		return &ExecutionGraph{ImmediateResponse: jsonStr}, nil
	}

	// Handle cases where the LLM returns an array directly instead of an object wrapping "spans"
	if strings.HasPrefix(strings.TrimSpace(jsonStr), "[") {
		var spans []Span
		if err := json.Unmarshal([]byte(jsonStr), &spans); err != nil {
			return nil, fmt.Errorf("failed to parse orchestration plan from LLM output (expected JSON array): %w\nRaw output:\n%s", err, jsonStr)
		}
		graph.Spans = spans
	} else {
		if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil {
			return nil, fmt.Errorf("failed to parse orchestration plan from LLM output (expected JSON object): %w\nRaw output:\n%s", err, jsonStr)
		}
	}

	return &graph, nil
}
