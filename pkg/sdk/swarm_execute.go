package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func (m *defaultSwarm) Execute(ctx context.Context, g *ExecutionGraph, o *Engine) (<-chan ObservableEvent, *Engine, error) {
	if o == nil {
		o = NewEngine(g)
	}
	m.activeEngine = o
	out := make(chan ObservableEvent, 1000)
	m.outChan = out
	if g != nil {
		o.AddSpans(g.Spans...)
		// Broadcast initial topology so UI can build the tree immediately
		for _, s := range g.Spans {
			out <- ObservableEvent{
				Timestamp: time.Now(),
				AgentName: s.Agent,
				SpanID:    s.ID,
				TaskName:  s.Name,
				ParentID:  s.ParentID,
				State:     AgentStatePending,
			}
		}
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(out)
		defer func() {
			if m.activeEngine == o {
				m.activeEngine = nil
				m.outChan = nil
			}
		}()
		type completedSpan struct {
			ID     string
			Result string
			Status SpanStatus
			Replan bool
		}
		completed := make(chan completedSpan, 100)
		activeSpans := 0
		var wg sync.WaitGroup
		replanCount := 0
		const maxReplans = 3

	Loop:
		for {
			ready := o.GetReadySpans()
			for _, t := range ready {
				o.MarkActive(t.ID)
				activeSpans++
				wg.Add(1)
				go func(span Span) {
					defer wg.Done()
					result, status, needsReplan := m.executeSpan(ctx, out, o, span)
					completed <- completedSpan{ID: span.ID, Result: result, Status: status, Replan: needsReplan}
				}(t)
			}

			if activeSpans == 0 {
				if o.IsComplete() {
					break Loop
				} else {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateError, Error: fmt.Errorf("deadlock detected in execution graph")}
					break Loop
				}
			}

			select {
			case <-ctx.Done():
				break Loop
			case done := <-completed:
				if done.Status == SpanStatusComplete {
					o.MarkComplete(done.ID, done.Result)
				} else {
					o.MarkFailed(done.ID)
				}
				activeSpans--

				if isDeadlocked, reason := o.IsDeadlocked(); isDeadlocked {
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Overwatch", State: AgentStateError, Error: fmt.Errorf("deadlock detected: %s", reason)}

					// Automatically commit this failure to semantic memory so it persists across sessions
					if m.memory != nil && m.memory.Semantic() != nil {
						_ = m.memory.Semantic().Commit(fmt.Sprintf("TOOL FAILURE (Overwatch): %s", reason))
					}

					// Force a hard replan and explicitly tell the planner WHY it failed so it routes around the block
					done.Replan = true
					done.Result = fmt.Sprintf("CRITICAL SYSTEM ERROR (OVERWATCH): %s You MUST find an alternative approach or ask the user for help.", reason)
					// reset the deadlock flag so the new plan can execute
					o.mu.Lock()
					o.deadlocked = false
					o.deadlockReason = ""
					o.mu.Unlock()
				}

				// Handle dynamic replanning or subgraph expansion
				if done.Replan {
					if replanCount >= maxReplans {
						errMsg := fmt.Errorf("maximum replan attempts reached, halting loop")
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateError, Error: errMsg}
						
						// Automated Post-Incident Artifact Generation
						traj := o.GetTrajectory()
						b, _ := json.MarshalIndent(traj, "", "  ")
						
						report := fmt.Sprintf("# Swarm Incident Report\n\n**Date:** %s\n**Session ID:** %s\n**Reason:** %v\n\n## Final Agent Error\n```\n%s\n```\n\n## Trajectory Dump\n```json\n%s\n```\n\n*Please review this report to identify tool failures or prompt loops. You can use the `quality_review` to analyze this file.*", time.Now().Format(time.RFC1123), m.sessionID, errMsg, done.Result, string(b))
						
						_ = os.WriteFile("incident_report.md", []byte(report), 0644)
						
						continue
					}
					replanCount++
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateThinking, Thought: fmt.Sprintf("Replanning effort %d/%d…", replanCount, maxReplans)}

					wg.Add(1)
					go func(failureResult string) {
						defer wg.Done()
						newG, err := m.Plan(ctx, "Pivot: "+failureResult, o.GetTrajectory())
						if err == nil {
							o.AddSpans(newG.Spans...)
							// Broadcast replanned topology so UI can build the tree immediately
							for _, s := range newG.Spans {
								out <- ObservableEvent{
									Timestamp: time.Now(),
									AgentName: s.Agent,
									SpanID:    s.ID,
									TaskName:  s.Name,
									ParentID:  s.ParentID,
									State:     AgentStatePending,
								}
							}
						}
					}(done.Result)
				}
			}
		}

		// Wait for all remaining active spans to recognize the context cancellation and gracefully exit
		wg.Wait()
	}()

	return out, o, nil
}

func (m *defaultSwarm) executeSpan(ctx context.Context, out chan<- ObservableEvent, o *Engine, span Span) (string, SpanStatus, bool) {
	m.mu.RLock()
	targetAgent, ok := m.agents[span.Agent]
	m.mu.RUnlock()

	if !ok {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("agent not found: %s", span.Agent)}
		return "Agent not found", SpanStatusFailed, false
	}

	// Use a unique session ID for this specific span
	spanSessionID := fmt.Sprintf("%s/%s", m.sessionID, span.ID)

	// To prevent SQLite "database is locked" errors under massive concurrency,
	// we use a lightweight in-memory session service for the sub-span runner.
	// The agent's interactions with the global blackboard (write_state tool)
	// will still correctly route to the global m.sessionSvc.
	spanSessionSvc := session.InMemoryService()

	_, err := spanSessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   "swarm-cli",
		UserID:    m.userID,
		SessionID: spanSessionID,
	})
	if err != nil {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("failed to initialize session for span: %w", err)}
		return "Session initialization failed", SpanStatusFailed, false
	}

	spanRunner, _ := runner.New(runner.Config{
		AppName:        "swarm-cli",
		Agent:          targetAgent,
		SessionService: spanSessionSvc,
	})

	out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateSpawning, Thought: "Starting…"}

	// Telemetry and Observation
	telemetryChan := make(chan string, 100)
	// Bind the telemetry channel context to the spanCtx so it cancels immediately when the runner returns
	spanCtx, cancel := context.WithCancel(context.WithValue(ctx, telemetryContextKey{}, (chan<- string)(telemetryChan)))
	defer cancel()

	telemetryDone := make(chan struct{})
	var obsWg sync.WaitGroup
	obsWg.Add(1)
	go func() {
		defer close(telemetryDone)
		defer obsWg.Done()
		var history []string
		var newLines bool

		var obsRunning bool
		var obsMu sync.Mutex

		baseInterval := 2 * time.Second
		interval := baseInterval
		maxInterval := 60 * time.Second
		timer := time.NewTimer(interval)
		defer timer.Stop()

		for {
			select {
			case <-spanCtx.Done(): // Fix: Use spanCtx instead of ctx to immediately stop the observer when the agent finishes
				return
			case line, ok := <-telemetryChan:
				if !ok {
					return
				}
				history = append(history, line)
				if len(history) > 100 {
					history = history[len(history)-100:]
				}
				newLines = true
			case <-timer.C:
				if newLines {
					obsMu.Lock()
					if obsRunning {
						obsMu.Unlock()
						timer.Reset(interval)
						continue
					}
					obsRunning = true
					obsMu.Unlock()

					newLines = false

					histCopy := make([]string, len(history))
					copy(histCopy, history)
					obsWg.Add(1)
					go func(hist []string) {
						defer obsWg.Done()
						defer func() {
							obsMu.Lock()
							obsRunning = false
							obsMu.Unlock()
						}()
						obsCtx, obsCancel := context.WithTimeout(spanCtx, 15*time.Second) // Fix: Tie observer timeout directly to spanCtx
						defer obsCancel()

						obsPrompt := fmt.Sprintf("Monitor: %s. Recent Telemetry:\n%s\n\nTask: Output a concise 3-8 word phrase summarizing the current activity (e.g., 'Searching for authentication logic...' or 'Running unit tests...'). If you detect an infinite loop or severe error, output 'INTERVENE: [reason]'.", targetAgent.Name(), strings.Join(hist, "\n"))

						respIter := m.fastModel.GenerateContent(obsCtx, &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText(obsPrompt, genai.Role("user"))}}, false)
						for resp, err := range respIter {
							if err != nil {
								out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Observer", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateWaiting, ObserverSummary: "Telemetry summary failed: " + err.Error()}
								break
							}
							if resp.Content != nil && len(resp.Content.Parts) > 0 {
								text := strings.TrimSpace(resp.Content.Parts[0].Text)
								if strings.HasPrefix(text, "INTERVENE:") {
									out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Observer", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateWaiting, ObserverSummary: text}
								} else {
									text = strings.TrimSuffix(text, ".")
									out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateExecuting, ObserverSummary: text}
								}
							}
							break
						}
					}(histCopy)

					interval = interval * 2
					if interval > maxInterval {
						interval = maxInterval
					}
				} else {
					// No new lines this cycle. Reset backoff so next burst is summarized quickly.
					interval = baseInterval
				}
				timer.Reset(interval)
			}
		}
	}()

	// Fetch and inject structured Session State (Node Autonomy coordination)
	var stateParts []string
	if sessResp, err := m.sessionSvc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: m.userID, SessionID: m.sessionID}); err == nil {
		sessionState := sessResp.Session.State()
		stateMap := make(map[string]any)
		for k, v := range sessionState.All() {
			stateMap[k] = v
		}
		if len(stateMap) > 0 {
			stateJSON, _ := json.MarshalIndent(stateMap, "", "  ")
			stateParts = append(stateParts, fmt.Sprintf("CURRENT SESSION STATE (Structured Data):\n%s", string(stateJSON)))
		}
	}

	// Inject results from dependencies into the prompt (Node Autonomy)
	contextMap := o.GetContext()
	var contextParts []string
	for _, depID := range span.Dependencies {
		if res, ok := contextMap[depID]; ok {
			if len(res) > 8000 {
				res = res[:8000] + "\n\n...[Content truncated to preserve context limits]..."
			}
			contextParts = append(contextParts, fmt.Sprintf("Output from previous span (%s):\n%s", depID, res))
		}
	}

	promptStr := fmt.Sprintf("TASK: %s\nINSTRUCTIONS: %s", span.Name, span.Prompt)
	promptStr += fmt.Sprintf("\n\n### TASK CONTEXT\nYour current Task ID is: %s\nIf you need to spawn subtasks, use this ID as their 'parent_id' or explicitly in their 'dependencies' list to ensure they block this task's completion if necessary.", span.ID)

	// Inject relevant semantic memory facts
	if m.memory != nil && m.memory.Semantic() != nil {
		if facts, err := m.memory.Semantic().Retrieve(span.Prompt, 3); err == nil && len(facts) > 0 {
			promptStr += "\n\n### RELEVANT MEMORY FACTS:\n" + strings.Join(facts, "\n")
		}
	}

	if len(stateParts) > 0 {
		promptStr = promptStr + "\n\n### SESSION STATE\n" + strings.Join(stateParts, "\n")
	}
	if len(contextParts) > 0 {
		promptStr = promptStr + "\n\n### RESULTS FROM PREVIOUS TASKS\n" + strings.Join(contextParts, "\n\n")
	}

	// Execute with the unique spanSessionID
	events := spanRunner.Run(spanCtx, m.userID, spanSessionID, genai.NewContentFromText(promptStr, genai.Role("user")), agent.RunConfig{})

	var full strings.Builder
	var needsReplan bool
	activeToolSpans := make(map[string][]Span)

	for event, err := range events {
		if err != nil {
			out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, Error: fmt.Errorf("error encountered: %w", err)}
			full.WriteString("\n\nERROR: " + err.Error())
			needsReplan = true
			break
		}
		var thoughtBroadcasted bool
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.FunctionCall != nil {
					if part.FunctionCall.Name == "request_replan" {
						needsReplan = true
					}
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateExecuting, ToolName: part.FunctionCall.Name, ToolArgs: part.FunctionCall.Args}

					// Record tool span start
					toolStart := time.Now()
					toolID := fmt.Sprintf("%s-tool-%s-%d", span.ID, part.FunctionCall.Name, toolStart.UnixNano())
					toolTask := Span{
						ID: toolID, ParentID: span.ID, TraceID: span.TraceID,
						Name: part.FunctionCall.Name, Kind: SpanKindTool, Status: SpanStatusActive,
						StartTime:  toolStart.Format(time.RFC3339Nano),
						Attributes: map[string]any{"gen_ai.tool_args": part.FunctionCall.Args},
					}
					o.AddSpans(toolTask)
					activeToolSpans[part.FunctionCall.Name] = append(activeToolSpans[part.FunctionCall.Name], toolTask)
				}
				if part.FunctionResponse != nil {
					var pgid int
					if val, ok := part.FunctionResponse.Response["pgid"]; ok {
						if num, ok := val.(float64); ok {
							pgid = int(num)
						} else if num, ok := val.(int); ok {
							pgid = num
						}
					}
					out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, ToolName: part.FunctionResponse.Name, PGID: pgid}

					// Record tool span completion (pop from queue)
					if spans, ok := activeToolSpans[part.FunctionResponse.Name]; ok && len(spans) > 0 {
						t := spans[0]
						activeToolSpans[part.FunctionResponse.Name] = spans[1:]

						now := time.Now()
						t.Status = SpanStatusComplete
						t.EndTime = now.Format(time.RFC3339Nano)
						if t.StartTime != "" {
							start, _ := time.Parse(time.RFC3339Nano, t.StartTime)
							t.Duration = now.Sub(start).String()
						}
						resJSON, _ := json.Marshal(part.FunctionResponse.Response)
						t.Attributes["gen_ai.tool_result"] = string(resJSON)
						o.AddSpans(t) // Update the span in the engine
					}
				}
				if part.Thought {
					if !thoughtBroadcasted {
						out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, Thought: "Thinking deeply..."}
						thoughtBroadcasted = true
					}
				} else if part.Text != "" {
					full.WriteString(part.Text)
				}
			}
		}
	}

	finalText := full.String()
	close(telemetryChan)
	<-telemetryDone
	obsWg.Wait()

	// Ensure all tool spans are closed (Node Autonomy cleanup)
	for _, spans := range activeToolSpans {
		for _, t := range spans {
			now := time.Now()
			t.Status = SpanStatusFailed
			t.EndTime = now.Format(time.RFC3339Nano)
			t.Attributes["gen_ai.tool_result"] = "Tool execution interrupted or incomplete"
			o.AddSpans(t)
		}
	}

	if finalText == "" {
		return "No response emitted by agent.", SpanStatusFailed, false
	}

	// Update the global state of the last agent to respond
	m.mu.Lock()
	m.lastAgent = targetAgent.Name()
	m.mu.Unlock()

	// If there was a hard error, skip Output Agent and immediately return with failed status and replan flag
	if strings.Contains(finalText, "\n\nERROR: ") || strings.Contains(finalText, "command not found") {
		m.appendEvent(ctx, "model", fmt.Sprintf("[%s]: %s", targetAgent.Name(), finalText))
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateError, FinalContent: finalText}
		return finalText, SpanStatusFailed, true
	}

	// Output Sanity Check (Output Agent)
	sanity := m.runOutputAgent(ctx, out, o, targetAgent.Name(), finalText)
	if sanity != outputRejected {
		// Record the response in the main conversation history
		m.appendEvent(ctx, "model", fmt.Sprintf("[%s]: %s", targetAgent.Name(), finalText))
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: targetAgent.Name(), SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateComplete, FinalContent: finalText}

		if sanity == outputNeedsInput {
			// If the agent explicitly asks the user a question, we must override any replan requests
			// and return normally so the engine halts and bubbles the question up to the user.
			return finalText, SpanStatusComplete, false
		}

		return finalText, SpanStatusComplete, needsReplan
	}

	out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", SpanID: span.ID, TaskName: span.Name, ParentID: span.ParentID, State: AgentStateThinking, Thought: "Output Agent rejected the response as problematic."}
	return "Output Agent rejected response.", SpanStatusComplete, true
}

type outputSanity int

const (
	AgentStatePending   AgentState = "pending"
	AgentStateSpawning  AgentState = "spawning"
	AgentStateThinking  AgentState = "thinking"
	AgentStateExecuting AgentState = "executing"
	AgentStateWaiting   AgentState = "waiting" // Blocked on HITL
	AgentStateComplete  AgentState = "complete"
	AgentStateError     AgentState = "error"
)
const (
	outputApproved outputSanity = iota
	outputRejected
	outputNeedsInput
)

func (m *defaultSwarm) runOutputAgent(ctx context.Context, out chan<- ObservableEvent, o *Engine, agentName, content string) outputSanity {
	coaStart := time.Now()
	coaPrompt := fmt.Sprintf("Sanity check worker: %s. Response: %s. RULE: Output ONLY 'OK', 'FIX: [reason]', or 'ASK_USER' if the agent explicitly asked the user a question requiring an answer before proceeding.", agentName, content)
	respIter := m.fastModel.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(coaPrompt, genai.Role("user"))},
		Config:   &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(m.outputInstruction, genai.Role("system"))},
	}, false)

	status := outputApproved
	var res string
	for resp, err := range respIter {
		if err == nil && resp.Content != nil && len(resp.Content.Parts) > 0 {
			res = strings.TrimSpace(resp.Content.Parts[0].Text)
			if strings.HasPrefix(res, "FIX:") {
				out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Output Agent", State: AgentStateError, FinalContent: "Rejected: " + res}
				status = outputRejected
			} else if strings.HasPrefix(res, "ASK_USER") {
				status = outputNeedsInput
			}
		}
		break
	}

	// Update UI card state
	if status == outputApproved {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Output Agent", State: AgentStateComplete, FinalContent: "OK"}
	} else if status == outputNeedsInput {
		out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Output Agent", State: AgentStateComplete, FinalContent: "Detected user question"}
	}

	if o != nil {
		o.AddSpans(Span{
			ID: "output-agent-" + fmt.Sprintf("%d", coaStart.UnixNano()), Name: "Sanity Check", Agent: "output_agent",
			Status: SpanStatusComplete, StartTime: coaStart.Format(time.RFC3339Nano), EndTime: time.Now().Format(time.RFC3339Nano),
			Duration: time.Since(coaStart).String(), Attributes: map[string]any{"gen_ai.prompt": coaPrompt, "gen_ai.completion": res},
		})
	}

	return status
}
