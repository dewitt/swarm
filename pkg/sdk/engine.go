package sdk

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// SpanStatus represents the current state of a span.
type SpanStatus string

const (
	SpanStatusPending     SpanStatus = "pending"
	SpanStatusActive      SpanStatus = "active"
	SpanStatusComplete    SpanStatus = "complete"
	SpanStatusBlocked     SpanStatus = "blocked"
	SpanStatusFailed      SpanStatus = "failed"
	SpanStatusInvalidated SpanStatus = "invalidated"
)

// SpanKind represents the type of operation in a trace.
type SpanKind string

const (
	SpanKindAgent   SpanKind = "agent"
	SpanKindTool    SpanKind = "tool"
	SpanKindPlanner SpanKind = "planning"
)

// Span (Span) represents a single unit of work aligned with OTel conventions.
type Span struct {
	ID           string            `json:"id"`
	TraceID      string            `json:"trace_id"`
	ParentID     string            `json:"parent_id,omitempty"`
	Name         string            `json:"operation_name"`
	Kind         SpanKind          `json:"kind"`
	Agent        string            `json:"agent,omitempty"`
	Attributes   map[string]any    `json:"attributes"` // OTel-style attributes (e.g. gen_ai.prompt)
	Status       SpanStatus        `json:"status"`
	StartTime    string            `json:"start_time"`
	EndTime      string            `json:"end_time,omitempty"`
	Duration     string            `json:"duration,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Result       string            `json:"result,omitempty"`
	Prompt       string            `json:"prompt,omitempty"` // The instructions for the agent
}

// ExecutionGraph represents a snapshot of the spans to be executed.
type ExecutionGraph struct {
	Spans             []Span `json:"spans"`
	ImmediateResponse string `json:"immediate_response,omitempty"` // Short-circuit for trivial requests
}

// Engine coordinates the reactive execution of a dynamic Span Pool.
type Engine struct {
	mu             sync.RWMutex
	traceID        string
	spans          map[string]Span
	status         map[string]SpanStatus
	result         map[string]string // Stores the "final response" of completed spans
	totalStartTime time.Time
}

// NewEngine creates a new Engine for a given initial seed graph.
func NewEngine(g *ExecutionGraph) *Engine {
	traceID := fmt.Sprintf("tr-%d", rand.Int63())
	o := &Engine{
		traceID:        traceID,
		spans:          make(map[string]Span),
		status:         make(map[string]SpanStatus),
		result:         make(map[string]string),
		totalStartTime: time.Now(),
	}
	if g != nil {
		o.AddSpans(g.Spans...)
	}
	return o
}

// AddSpans adds new spans to the pool and ensures OTel metadata is set.
func (o *Engine) AddSpans(spans ...Span) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, t := range spans {
		if t.TraceID == "" {
			t.TraceID = o.traceID
		}
		if t.Attributes == nil {
			t.Attributes = make(map[string]any)
		}
		if t.Prompt != "" {
			t.Attributes["gen_ai.prompt"] = t.Prompt
		}
		
		// Record the span result if it's already complete
		if t.Status == SpanStatusComplete {
			if res, ok := t.Attributes["gen_ai.completion"].(string); ok {
				o.result[t.ID] = res
			}
		}

		o.spans[t.ID] = t
		
		// Always update status if provided, otherwise default to Pending
		if t.Status != "" {
			o.status[t.ID] = t.Status
		} else if _, exists := o.status[t.ID]; !exists {
			o.status[t.ID] = SpanStatusPending
		}

		// Sanity check: if any dependency is already failed or invalidated,
		// this new span should be invalidated immediately.
		for _, depID := range t.Dependencies {
			if s, exists := o.status[depID]; exists && (s == SpanStatusFailed || s == SpanStatusInvalidated) {
				o.status[t.ID] = SpanStatusInvalidated
				t.Status = SpanStatusInvalidated
				o.spans[t.ID] = t
				o.invalidateDependentsLocked(t.ID)
				break
			}
		}
	}
}

// GetReadySpans returns spans that are pending and have met all dependencies.
func (o *Engine) GetReadySpans() []Span {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var ready []Span
	for id, t := range o.spans {
		if o.status[id] != SpanStatusPending {
			continue
		}

		allDepsMet := true
		for _, depID := range t.Dependencies {
			// If dependency exists and isn't complete, we aren't ready.
			// If it doesn't exist, we treat it as met (pruning the broken link).
			if s, exists := o.status[depID]; exists && s != SpanStatusComplete {
				allDepsMet = false
				break
			}
		}

		if allDepsMet {
			ready = append(ready, t)
		}
	}
	return ready
}

// MarkActive marks a span as actively executing.
func (o *Engine) MarkActive(spanID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[spanID] = SpanStatusActive
	t := o.spans[spanID]
	t.StartTime = time.Now().Format(time.RFC3339Nano)
	o.spans[spanID] = t
}

// MarkComplete marks a span as successfully completed and stores its result.
func (o *Engine) MarkComplete(spanID string, result string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[spanID] = SpanStatusComplete
	o.result[spanID] = result
	
	t := o.spans[spanID]
	now := time.Now()
	t.EndTime = now.Format(time.RFC3339Nano)
	if t.StartTime != "" {
		start, _ := time.Parse(time.RFC3339Nano, t.StartTime)
		t.Duration = now.Sub(start).String()
	}
	if t.Attributes == nil {
		t.Attributes = make(map[string]any)
	}
	t.Attributes["gen_ai.completion"] = result
	t.Status = SpanStatusComplete
	o.spans[spanID] = t
}

// MarkFailed marks a span as failed and invalidates its dependents.
func (o *Engine) MarkFailed(spanID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	o.status[spanID] = SpanStatusFailed
	t := o.spans[spanID]
	t.Status = SpanStatusFailed
	o.spans[spanID] = t

	// Recursively invalidate all dependents to prevent deadlock
	o.invalidateDependentsLocked(spanID)
}

// invalidateDependentsLocked prunes all branches that depend on the given span.
// Must be called while holding the lock.
func (o *Engine) invalidateDependentsLocked(spanID string) {
	for id, t := range o.spans {
		for _, depID := range t.Dependencies {
			if depID == spanID {
				if o.status[id] != SpanStatusInvalidated {
					o.status[id] = SpanStatusInvalidated
					span := o.spans[id]
					span.Status = SpanStatusInvalidated
					o.spans[id] = span
					// Recurse
					o.invalidateDependentsLocked(id)
				}
				break
			}
		}
	}
}

// MarkInvalidated prunes a span or branch.
func (o *Engine) MarkInvalidated(spanID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[spanID] = SpanStatusInvalidated
	t := o.spans[spanID]
	t.Status = SpanStatusInvalidated
	o.spans[spanID] = t
}

// IsComplete returns true if all non-invalidated spans are complete.
func (o *Engine) IsComplete() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for id := range o.spans {
		s := o.status[id]
		if s != SpanStatusComplete && s != SpanStatusInvalidated && s != SpanStatusFailed {
			return false
		}
	}
	return true
}

// GetContext returns a synthesis of completed span results for injection into new spans.
func (o *Engine) GetContext() map[string]string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	// Deep copy of results
	ctx := make(map[string]string)
	for k, v := range o.result {
		ctx[k] = v
	}
	return ctx
}

type Trajectory struct {
	TraceID       string `json:"trace_id"`
	Spans         []Span `json:"spans"`
	TotalDuration string `json:"total_duration"`
}

// GetTrajectory returns a full timed trajectory of the swarm execution.
func (o *Engine) GetTrajectory() Trajectory {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	var spans []Span
	for _, t := range o.spans {
		spans = append(spans, t)
	}
	
	return Trajectory{
		TraceID:       o.traceID,
		Spans:         spans,
		TotalDuration: time.Since(o.totalStartTime).String(),
	}
}
