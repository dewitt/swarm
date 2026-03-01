package sdk

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusActive      TaskStatus = "active"
	TaskStatusComplete    TaskStatus = "complete"
	TaskStatusBlocked     TaskStatus = "blocked"
	TaskStatusFailed      TaskStatus = "failed"
	TaskStatusInvalidated TaskStatus = "invalidated"
)

// SpanKind represents the type of operation in a trace.
type SpanKind string

const (
	SpanKindAgent   SpanKind = "agent"
	SpanKindTool    SpanKind = "tool"
	SpanKindPlanner SpanKind = "planning"
)

// Task (Span) represents a single unit of work aligned with OTel conventions.
type Task struct {
	ID           string            `json:"id"`
	TraceID      string            `json:"trace_id"`
	ParentID     string            `json:"parent_id,omitempty"`
	Name         string            `json:"operation_name"`
	Kind         SpanKind          `json:"kind"`
	Agent        string            `json:"agent,omitempty"`
	Attributes   map[string]any    `json:"attributes"` // OTel-style attributes (e.g. gen_ai.prompt)
	Status       TaskStatus        `json:"status"`
	StartTime    string            `json:"start_time"`
	EndTime      string            `json:"end_time,omitempty"`
	Duration     string            `json:"duration,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Result       string            `json:"result,omitempty"`
	Prompt       string            `json:"prompt,omitempty"` // The instructions for the agent
}

// ExecutionGraph represents a snapshot of the tasks to be executed.
type ExecutionGraph struct {
	Tasks             []Task `json:"tasks"`
	ImmediateResponse string `json:"immediate_response,omitempty"` // Short-circuit for trivial requests
}

// Orchestrator coordinates the reactive execution of a dynamic Task Pool.
type Orchestrator struct {
	mu             sync.RWMutex
	traceID        string
	tasks          map[string]Task
	status         map[string]TaskStatus
	result         map[string]string // Stores the "final response" of completed tasks
	totalStartTime time.Time
}

// NewOrchestrator creates a new Orchestrator for a given initial seed graph.
func NewOrchestrator(g *ExecutionGraph) *Orchestrator {
	traceID := fmt.Sprintf("tr-%d", rand.Int63())
	o := &Orchestrator{
		traceID:        traceID,
		tasks:          make(map[string]Task),
		status:         make(map[string]TaskStatus),
		result:         make(map[string]string),
		totalStartTime: time.Now(),
	}
	if g != nil {
		o.AddTasks(g.Tasks...)
	}
	return o
}

// AddTasks adds new tasks to the pool and ensures OTel metadata is set.
func (o *Orchestrator) AddTasks(tasks ...Task) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, t := range tasks {
		if t.TraceID == "" {
			t.TraceID = o.traceID
		}
		if t.Attributes == nil {
			t.Attributes = make(map[string]any)
		}
		if t.Prompt != "" {
			t.Attributes["gen_ai.prompt"] = t.Prompt
		}
		
		// Record the task result if it's already complete
		if t.Status == TaskStatusComplete {
			if res, ok := t.Attributes["gen_ai.completion"].(string); ok {
				o.result[t.ID] = res
			}
		}

		o.tasks[t.ID] = t
		
		// Always update status if provided, otherwise default to Pending
		if t.Status != "" {
			o.status[t.ID] = t.Status
		} else if _, exists := o.status[t.ID]; !exists {
			o.status[t.ID] = TaskStatusPending
		}

		// Sanity check: if any dependency is already failed or invalidated,
		// this new task should be invalidated immediately.
		for _, depID := range t.Dependencies {
			if s, exists := o.status[depID]; exists && (s == TaskStatusFailed || s == TaskStatusInvalidated) {
				o.status[t.ID] = TaskStatusInvalidated
				t.Status = TaskStatusInvalidated
				o.tasks[t.ID] = t
				o.invalidateDependentsLocked(t.ID)
				break
			}
		}
	}
}

// GetReadyTasks returns tasks that are pending and have met all dependencies.
func (o *Orchestrator) GetReadyTasks() []Task {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var ready []Task
	for id, t := range o.tasks {
		if o.status[id] != TaskStatusPending {
			continue
		}

		allDepsMet := true
		for _, depID := range t.Dependencies {
			// If dependency exists and isn't complete, we aren't ready.
			// If it doesn't exist, we treat it as met (pruning the broken link).
			if s, exists := o.status[depID]; exists && s != TaskStatusComplete {
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

// MarkActive marks a task as actively executing.
func (o *Orchestrator) MarkActive(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[taskID] = TaskStatusActive
	t := o.tasks[taskID]
	t.StartTime = time.Now().Format(time.RFC3339Nano)
	o.tasks[taskID] = t
}

// MarkComplete marks a task as successfully completed and stores its result.
func (o *Orchestrator) MarkComplete(taskID string, result string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[taskID] = TaskStatusComplete
	o.result[taskID] = result
	
	t := o.tasks[taskID]
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
	t.Status = TaskStatusComplete
	o.tasks[taskID] = t
}

// MarkFailed marks a task as failed and invalidates its dependents.
func (o *Orchestrator) MarkFailed(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	o.status[taskID] = TaskStatusFailed
	t := o.tasks[taskID]
	t.Status = TaskStatusFailed
	o.tasks[taskID] = t

	// Recursively invalidate all dependents to prevent deadlock
	o.invalidateDependentsLocked(taskID)
}

// invalidateDependentsLocked prunes all branches that depend on the given task.
// Must be called while holding the lock.
func (o *Orchestrator) invalidateDependentsLocked(taskID string) {
	for id, t := range o.tasks {
		for _, depID := range t.Dependencies {
			if depID == taskID {
				if o.status[id] != TaskStatusInvalidated {
					o.status[id] = TaskStatusInvalidated
					task := o.tasks[id]
					task.Status = TaskStatusInvalidated
					o.tasks[id] = task
					// Recurse
					o.invalidateDependentsLocked(id)
				}
				break
			}
		}
	}
}

// MarkInvalidated prunes a task or branch.
func (o *Orchestrator) MarkInvalidated(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[taskID] = TaskStatusInvalidated
	t := o.tasks[taskID]
	t.Status = TaskStatusInvalidated
	o.tasks[taskID] = t
}

// IsComplete returns true if all non-invalidated tasks are complete.
func (o *Orchestrator) IsComplete() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for id := range o.tasks {
		s := o.status[id]
		if s != TaskStatusComplete && s != TaskStatusInvalidated && s != TaskStatusFailed {
			return false
		}
	}
	return true
}

// GetContext returns a synthesis of completed task results for injection into new tasks.
func (o *Orchestrator) GetContext() map[string]string {
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
	Spans         []Task `json:"spans"`
	TotalDuration string `json:"total_duration"`
}

// GetTrajectory returns a full timed trajectory of the swarm execution.
func (o *Orchestrator) GetTrajectory() Trajectory {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	var spans []Task
	for _, t := range o.tasks {
		spans = append(spans, t)
	}
	
	return Trajectory{
		TraceID:       o.traceID,
		Spans:         spans,
		TotalDuration: time.Since(o.totalStartTime).String(),
	}
}
