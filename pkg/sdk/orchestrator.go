package sdk

// Task represents a single unit of work in a multi-agent execution graph.
type Task struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Agent        string   `json:"agent"`        // The specific agent/skill capable of this task
	Prompt       string   `json:"prompt"`       // The prompt or instructions for the agent
	Dependencies []string `json:"dependencies"` // IDs of tasks that must complete before this one starts
}

// ExecutionGraph represents the DAG of tasks to be executed by the swarm.
type ExecutionGraph struct {
	Tasks []Task `json:"tasks"`
}

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusActive   TaskStatus = "active"
	TaskStatusComplete TaskStatus = "complete"
	TaskStatusBlocked  TaskStatus = "blocked"
	TaskStatusFailed   TaskStatus = "failed"
)

// Orchestrator coordinates the parallel execution of an ExecutionGraph.
type Orchestrator struct {
	graph  *ExecutionGraph
	status map[string]TaskStatus
}

// NewOrchestrator creates a new Orchestrator for a given graph.
func NewOrchestrator(g *ExecutionGraph) *Orchestrator {
	status := make(map[string]TaskStatus)
	for _, t := range g.Tasks {
		status[t.ID] = TaskStatusPending
	}
	return &Orchestrator{
		graph:  g,
		status: status,
	}
}

// GetReadyTasks returns a list of tasks that are ready to be executed
// (i.e. they are pending and all their dependencies are complete).
func (o *Orchestrator) GetReadyTasks() []Task {
	var ready []Task
	for _, t := range o.graph.Tasks {
		if o.status[t.ID] != TaskStatusPending {
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
	o.status[taskID] = TaskStatusActive
}

// MarkComplete marks a task as successfully completed.
func (o *Orchestrator) MarkComplete(taskID string) {
	o.status[taskID] = TaskStatusComplete
}

// MarkFailed marks a task as failed.
func (o *Orchestrator) MarkFailed(taskID string) {
	o.status[taskID] = TaskStatusFailed
}

// IsComplete returns true if all tasks in the graph are complete.
func (o *Orchestrator) IsComplete() bool {
	for _, status := range o.status {
		if status != TaskStatusComplete {
			return false
		}
	}
	return true
}
