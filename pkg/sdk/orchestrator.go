package sdk

import (
	"sync"
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

// Task represents a single unit of work in a multi-agent execution graph.
type Task struct {
	ID           string   `json:"id"`
	ParentID     string   `json:"parent_id,omitempty"` // ID of the task that spawned this one
	Name         string   `json:"name"`
	Agent        string   `json:"agent"`        // The specific agent/skill capable of this task
	Prompt       string   `json:"prompt"`       // The prompt or instructions for the agent
	Dependencies []string `json:"dependencies"` // IDs of tasks that must complete before this one starts
}

// ExecutionGraph represents a snapshot of the tasks to be executed.
type ExecutionGraph struct {
	Tasks             []Task `json:"tasks"`
	ImmediateResponse string `json:"immediate_response,omitempty"` // Short-circuit for trivial requests
}

// Orchestrator coordinates the reactive execution of a dynamic Task Pool.
type Orchestrator struct {
	mu     sync.RWMutex
	tasks  map[string]Task
	status map[string]TaskStatus
	result map[string]string // Stores the "final response" of completed tasks
}

// NewOrchestrator creates a new Orchestrator for a given initial seed graph.
func NewOrchestrator(g *ExecutionGraph) *Orchestrator {
	o := &Orchestrator{
		tasks:  make(map[string]Task),
		status: make(map[string]TaskStatus),
		result: make(map[string]string),
	}
	if g != nil {
		o.AddTasks(g.Tasks...)
	}
	return o
}

// AddTasks adds new tasks to the pool.
func (o *Orchestrator) AddTasks(tasks ...Task) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, t := range tasks {
		o.tasks[t.ID] = t
		if _, exists := o.status[t.ID]; !exists {
			o.status[t.ID] = TaskStatusPending
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
			if o.status[depID] != TaskStatusComplete {
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
}

// MarkComplete marks a task as successfully completed and stores its result.
func (o *Orchestrator) MarkComplete(taskID string, result string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[taskID] = TaskStatusComplete
	o.result[taskID] = result
}

// MarkFailed marks a task as failed.
func (o *Orchestrator) MarkFailed(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[taskID] = TaskStatusFailed
}

// MarkInvalidated prunes a task or branch.
func (o *Orchestrator) MarkInvalidated(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status[taskID] = TaskStatusInvalidated
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
