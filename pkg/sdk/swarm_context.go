package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/adk/tool"
)

func (m *defaultSwarm) readState(ctx tool.Context, req struct{ Key string }) (string, error) {
	if m.memory == nil || m.memory.Episodic() == nil {
		return "", fmt.Errorf("episodic memory not initialized")
	}
	val, err := m.memory.Episodic().GetState(context.Background(), m.sessionID, req.Key)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(val)
	return string(b), nil
}

func (m *defaultSwarm) writeState(ctx tool.Context, req struct {
	Key   string
	Value any
},
) (string, error) {
	if m.memory == nil || m.memory.Episodic() == nil {
		return "", fmt.Errorf("episodic memory not initialized")
	}
	err := m.memory.Episodic().SetState(context.Background(), m.sessionID, req.Key, req.Value)
	if err != nil {
		return "", err
	}
	return "State updated successfully.", nil
}

func (m *defaultSwarm) retrieveFact(ctx tool.Context, req struct {
	Query string
	Limit int
},
) (string, error) {
	if m.memory == nil || m.memory.Semantic() == nil {
		return "", fmt.Errorf("semantic memory not initialized")
	}

	limit := req.Limit
	if limit == 0 {
		limit = 5
	}

	facts, err := m.memory.Semantic().Retrieve(req.Query, limit)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve facts: %w", err)
	}

	if len(facts) == 0 {
		return "No relevant facts found in semantic memory.", nil
	}

	b, _ := json.MarshalIndent(facts, "", "  ")
	return string(b), nil
}

func (m *defaultSwarm) spawnSubtask(ctx tool.Context, req struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Agent        string   `json:"agent"`
	Prompt       string   `json:"prompt"`
	Dependencies []string `json:"dependencies"`
	ParentID     string   `json:"parent_id,omitempty"`
},
) (string, error) {
	if m.activeEngine == nil {
		return "", fmt.Errorf("no active execution engine")
	}

	// Prefix the new ID with the ParentID to ensure uniqueness if needed,
	// or let the agent handle it. Let's just use what they provide.
	span := Span{
		ID:           req.ID,
		Name:         req.Name,
		Agent:        req.Agent,
		Prompt:       req.Prompt,
		Dependencies: req.Dependencies,
		ParentID:     req.ParentID,
		Kind:         SpanKindAgent,
		Status:       SpanStatusPending,
	}
	m.activeEngine.AddSpans(span)

	if m.outChan != nil {
		m.outChan <- ObservableEvent{
			Timestamp: time.Now(),
			AgentName: req.Agent,
			SpanID:    req.ID,
			TaskName:  req.Name,
			ParentID:  req.ParentID,
			State:     AgentStatePending,
		}
	}

	return fmt.Sprintf("Subtask '%s' (%s) successfully spawned and added to the execution graph.", req.Name, req.ID), nil
}

func (m *defaultSwarm) AddContext(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m.pinnedContext[path] = string(b)
	return nil
}

func (m *defaultSwarm) DropContext(path string) {
	if path == "all" {
		m.pinnedContext = make(map[string]string)
	} else {
		delete(m.pinnedContext, path)
	}
}

func (m *defaultSwarm) ListContext() []string {
	var p []string
	for path := range m.pinnedContext {
		p = append(p, path)
	}
	return p
}

func (m *defaultSwarm) appendEvent(ctx context.Context, author, content string) {
	if m.memory != nil && m.memory.Episodic() != nil {
		_ = m.memory.Episodic().AppendEvent(ctx, m.sessionID, author, content)
	}
}

func (m *defaultSwarm) Rewind(n int) error {
	if n <= 0 {
		return nil
	}
	var evs []struct{ Timestamp string }
	err := m.db.Table("events").Select("timestamp").Where("session_id = ? AND author = ?", m.sessionID, "user").Order("timestamp DESC").Limit(n).Find(&evs).Error
	if err != nil {
		return err
	}
	if len(evs) < n {
		return m.db.Table("events").Where("session_id = ?", m.sessionID).Delete(nil).Error
	}
	return m.db.Table("events").Where("session_id = ? AND timestamp >= ?", m.sessionID, evs[len(evs)-1].Timestamp).Delete(nil).Error
}

func (m *defaultSwarm) saveTrajectory(traj Trajectory) {
	baseDir, err := GetConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(baseDir, m.trajectoryDir)
	_ = os.MkdirAll(dir, 0o755)

	filename := fmt.Sprintf("%s.json", traj.TraceID)
	if m.sessionID != "" {
		filename = fmt.Sprintf("%s_%s.json", m.sessionID, traj.TraceID)
	}
	// Sanitize output path
	filename = strings.ReplaceAll(filename, "/", "_")
	path := filepath.Join(dir, filename)

	if m.telemetryConfigured || m.forceDonate {
		// Convert the typed struct into a generic map so we can inject the "donate" field at the root level without altering the core schema
		b, err := json.Marshal(traj)
		if err == nil {
			var dynMap map[string]interface{}
			if err := json.Unmarshal(b, &dynMap); err == nil {
				dynMap["donate"] = true
				b, _ = json.MarshalIndent(dynMap, "", "  ")
				_ = os.WriteFile(path, b, 0o600)
				return
			}
		}
	}

	b, _ := json.MarshalIndent(traj, "", "  ")
	_ = os.WriteFile(path, b, 0o600)
}
