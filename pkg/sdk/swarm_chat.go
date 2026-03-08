package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func (m *defaultSwarm) Chat(ctx context.Context, prompt string) (<-chan ObservableEvent, error) {
	out := make(chan ObservableEvent, 1000)
	// Record the user's initial prompt in the persistent session
	m.appendEvent(ctx, "user", prompt)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(out)
		swarmStartTime := time.Now()
		o := NewEngine(nil)

		defer func() {
			if o != nil {
				traj := o.GetTrajectory()
				traj.TotalDuration = time.Since(swarmStartTime).String()
				m.saveTrajectory(traj)

				if m.debugMode {
					b, _ := json.MarshalIndent(traj, "", "  ")
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateComplete, FinalContent: string(b)}
				}
			}
		}()

		// 1. Input Classification (Fast Path / CIA)
		inputStart := time.Now()
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateThinking, Thought: "Classifying intent…"}

		// Dynamically inject the last active agent to help the CIA detect digressions
		m.mu.RLock()
		lastAgentName := "None (Starting new conversation)"
		if m.lastAgent != "" {
			lastAgentName = m.lastAgent
		}

		var descriptions []string
		for _, name := range m.subAgentNames {
			if a, ok := m.agents[name]; ok {
				descriptions = append(descriptions, fmt.Sprintf("- **%s**: %s", name, a.Description()))
			}
		}
		specialistsList := strings.Join(descriptions, "\n")
		inputInstruction := m.inputInstruction
		m.mu.RUnlock()

		// Fetch and inject global conversation history for the Input Agent
		var inputHistoryParts []string
		if m.memory != nil && m.memory.Episodic() != nil {
			if hist, err := m.memory.Episodic().GetRecentHistory(ctx, m.sessionID, 5); err == nil {
				inputHistoryParts = hist
			}
		}

		dynamicInstruction := inputInstruction + fmt.Sprintf("\n\nAVAILABLE AGENTS:\n%s\n\nCURRENT CONTEXT: The last agent to respond was: %s.", specialistsList, lastAgentName)
		if len(inputHistoryParts) > 0 {
			// Only take the last 5 turns to keep the prompt small and fast
			if len(inputHistoryParts) > 5 {
				inputHistoryParts = inputHistoryParts[len(inputHistoryParts)-5:]
			}
			dynamicInstruction += "\n\nRECENT CONVERSATION HISTORY:\n" + strings.Join(inputHistoryParts, "\n")
		}

		// Inject relevant semantic memory facts for the Input Agent so it can short-circuit to Swarm if the answer is known
		if m.memory != nil && m.memory.Semantic() != nil {
			if facts, err := m.memory.Semantic().Retrieve(prompt, 3); err == nil && len(facts) > 0 {
				dynamicInstruction += "\n\n### RELEVANT SYSTEM FACTS (SEMANTIC MEMORY):\n" + strings.Join(facts, "\n")
			}
		}

		inputIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
			Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.Role("user"))},
			Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(dynamicInstruction, genai.Role("system"))},
		}, false)

		var inputResult string
		for resp, err := range inputIter {
			if err != nil {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateError, Error: fmt.Errorf("input classification failed: %w", err)}
				return
			}
			if resp.Content != nil && len(resp.Content.Parts) > 0 {
				inputResult = resp.Content.Parts[0].Text
			}
			break
		}

		o.AddSpans(Span{
			ID: "input", Name: "Classification", Agent: "input_agent", Status: SpanStatusComplete,
			Kind:      SpanKindPlanner,
			StartTime: inputStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
			Duration: time.Since(inputStart).String(), Attributes: map[string]any{"gen_ai.prompt": prompt, "gen_ai.completion": inputResult},
		})

		var target string
		if strings.HasPrefix(inputResult, "ROUTE TO: swarm_agent") {
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateExecuting, Thought: "Rerouting to Swarm…"}
			target = "swarm_agent"
		} else {
			target = "swarm_agent"
			m.mu.RLock()
			if m.lastAgent != "" {
				target = m.lastAgent
			}
			m.mu.RUnlock()
		}
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Input Agent", SpanID: "input", TaskName: "Classification", State: AgentStateComplete, FinalContent: "Routed to " + target}

		originalPrompt := prompt
		cyclePrompt := prompt

		maxCycles := 5
		for cycle := 0; cycle < maxCycles; cycle++ {
			var graph *ExecutionGraph
			var err error

			// 2. Swarm Coordination / Planning
			// If we are starting fresh or rerouting to Swarm Agent, we let it plan.
			if target == "swarm_agent" {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateThinking, Thought: "Analyzing request…"}
				planStart := time.Now()

				// Inject the same history parts into the planner if available
				plannerPrompt := cyclePrompt
				if cycle == 0 && len(inputHistoryParts) > 0 {
					plannerPrompt += "\n\n### RECENT CONVERSATION HISTORY (For Context):\n" + strings.Join(inputHistoryParts, "\n")
				}

				graph, err = m.Plan(ctx, plannerPrompt, o.GetTrajectory())
				if err != nil {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateError, Error: fmt.Errorf("coordination failed: %w", err)}
					return
				}

				if graph != nil && graph.ImmediateResponse == "" {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateComplete, FinalContent: "Delegated tasks to sub-agents."}
				}

				planJSON, _ := json.Marshal(graph)
				o.AddSpans(Span{
					ID: "coordination", Name: "Swarm Planning", Agent: "swarm_agent", Status: SpanStatusComplete,
					Kind:      SpanKindPlanner,
					StartTime: planStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
					Duration: time.Since(planStart).String(), Attributes: map[string]any{"gen_ai.prompt": cyclePrompt, "gen_ai.completion": string(planJSON)},
				})
			} else {
				// Direct execution with specialized agent (Node Autonomy)
				graph = &ExecutionGraph{Spans: []Span{{ID: "t1", Name: "Fulfill", Agent: target, Prompt: cyclePrompt}}}
			}

			// Namespace spans if this is a reflection cycle
			if cycle > 0 && graph != nil {
				prefix := fmt.Sprintf("c%d-", cycle)
				for i := range graph.Spans {
					graph.Spans[i].ID = prefix + graph.Spans[i].ID
					if graph.Spans[i].ParentID != "" {
						graph.Spans[i].ParentID = prefix + graph.Spans[i].ParentID
					}
					for j := range graph.Spans[i].Dependencies {
						graph.Spans[i].Dependencies[j] = prefix + graph.Spans[i].Dependencies[j]
					}
				}
			}

			// 3. Execution
			if len(graph.Spans) > 0 {
				events, updatedEngine, err := m.Execute(ctx, graph, o)
				if err != nil {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "execution", TaskName: "Graph Execution", State: AgentStateError, Error: fmt.Errorf("execution failed: %w", err)}
					return
				}
				for event := range events {
					out <- event
				}
				o = updatedEngine

				// 4. Reflect
				if target == "swarm_agent" {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateThinking, Thought: "Evaluating if original goal is complete..."}

					reflection, err := m.Reflect(ctx, originalPrompt, o.GetTrajectory())
					if err != nil {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateError, Error: fmt.Errorf("reflection failed: %w", err)}
						return
					}

					// Automatically commit any new facts discovered during reflection
					if m.memory != nil && m.memory.Semantic() != nil && len(reflection.NewFacts) > 0 {
						for _, fact := range reflection.NewFacts {
							_ = m.memory.Semantic().Commit(fact)
						}
					}

					// Passively compress the episodic working memory now that facts are extracted
					o.Prune()

					if reflection.NeedsUserInput {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateComplete, Thought: "Waiting for user input.", FinalContent: reflection.Reasoning}
						break
					} else if reflection.IsResolved {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateComplete, Thought: "Goal satisfied."}
						break
					} else {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "reflection", TaskName: "Reflection", State: AgentStateComplete, Thought: "Goal not satisfied yet. Replanning."}
						cyclePrompt = fmt.Sprintf("Original Goal: %s\n\nReflection from last cycle: %s\nNext Steps: %s", originalPrompt, reflection.Reasoning, reflection.NextSteps)
					}
				} else {
					break // Direct routes exit immediately
				}
			} else if graph.ImmediateResponse != "" {
				sanity := m.runOutputAgent(ctx, out, o, "Swarm", graph.ImmediateResponse)
				if sanity != outputRejected {
					// Record the immediate response in the persistent session
					m.appendEvent(ctx, "model", graph.ImmediateResponse)
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: "coordination", TaskName: "Swarm Planning", State: AgentStateComplete, FinalContent: graph.ImmediateResponse}
				}
				break
			} else {
				break
			}
		}

		// 5. End of Chat
	}()
	return out, nil
}
