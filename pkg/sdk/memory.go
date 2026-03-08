package sdk

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// ... (keep interfaces as they were)
// HierarchicalMemory represents the 4-Tier Memory System for context management.
// It orchestrates how information flows across different temporal scopes.
type HierarchicalMemory interface {
	// Working Memory (Tier 1): The ephemeral execution state of the current cycle.
	Working() WorkingMemory

	// Episodic Memory (Tier 2): The chronological audit log of a given session.
	Episodic() EpisodicMemory

	// Semantic Memory (Tier 3): Persistent, timeless facts local to the workspace.
	Semantic() SemanticMemory

	// Global Memory (Tier 4): Foundational parameters and cross-project preferences.
	Global() GlobalMemory
}

// MemoryStats provides metadata about a specific memory tier.
type MemoryStats struct {
	Name          string
	Count         int
	TokenEstimate int
}

// WorkingMemory provides access to the current active execution context (Tier 1).
type WorkingMemory interface {
	// AddSpans records new execution spans.
	AddSpans(spans ...Span)

	// GetTrajectory returns the full execution trajectory of the current cycle.
	GetTrajectory() Trajectory

	// GetContext returns a synthesis of completed span results.
	GetContext() map[string]string

	// WorkingStats returns metadata about working memory.
	WorkingStats() MemoryStats
}

// EpisodicMemory provides access to session-scoped state and chronological logs (Tier 2).
type EpisodicMemory interface {
	// GetState retrieves a specific key from the session state.
	GetState(ctx context.Context, sessionID, key string) (any, error)

	// SetState updates a specific key in the session state.
	SetState(ctx context.Context, sessionID, key string, value any) error

	// AppendEvent records a new event in the session log.
	AppendEvent(ctx context.Context, sessionID, author, content string) error

	// GetRecentHistory retrieves the most recent conversation history for a session.
	GetRecentHistory(ctx context.Context, sessionID string, limit int) ([]string, error)

	// EpisodicStats returns metadata about episodic memory for a specific session.
	EpisodicStats(ctx context.Context, sessionID string) MemoryStats
}

// SemanticMemory represents persistent, workspace-local facts (Tier 3).
type SemanticMemory interface {
	// Commit persistently stores a new fact.
	Commit(fact string) error

	// Retrieve queries for facts relevant to the given query.
	Retrieve(query string, limit int) ([]string, error)

	// List returns the most recently committed facts.
	List(limit int) ([]string, error)

	// Forget removes any facts from semantic memory that contain the given keyword.
	Forget(query string) (int, error)

	// SemanticStats returns metadata about semantic memory.
	SemanticStats() MemoryStats

	// FTSEnabled returns true if the semantic memory is backed by FTS5.
	FTSEnabled() bool
}

// GlobalMemory provides access to system-wide parameters and preferences (Tier 4).
type GlobalMemory interface {
	// Load retrieves all globally stored memory.
	Load() (string, error)

	// Save appends a new fact or preference globally.
	Save(fact string) error

	// GlobalStats returns metadata about global memory.
	GlobalStats() MemoryStats
}

// episodicMemoryImpl implements EpisodicMemory backed by an ADK session.Service
type episodicMemoryImpl struct {
	svc    session.Service
	userID string
}

func NewEpisodicMemory(svc session.Service, userID string) EpisodicMemory {
	return &episodicMemoryImpl{
		svc:    svc,
		userID: userID,
	}
}

func (e *episodicMemoryImpl) GetState(ctx context.Context, sessionID, key string) (any, error) {
	resp, err := e.svc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: e.userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	return resp.Session.State().Get(key)
}

func (e *episodicMemoryImpl) SetState(ctx context.Context, sessionID, key string, value any) error {
	resp, err := e.svc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: e.userID, SessionID: sessionID})
	if err != nil {
		return err
	}
	err = resp.Session.State().Set(key, value)
	if err != nil {
		return err
	}
	// Trigger save
	ev := session.NewEvent("")
	ev.Author = "System"
	ev.LLMResponse = model.LLMResponse{
		Content: genai.NewContentFromText(fmt.Sprintf("State updated: %s", key), genai.Role("system")),
	}
	ev.Actions = session.EventActions{StateDelta: map[string]any{key: value}}

	return e.svc.AppendEvent(ctx, resp.Session, ev)
}

func (e *episodicMemoryImpl) AppendEvent(ctx context.Context, sessionID, author, content string) error {
	resp, err := e.svc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: e.userID, SessionID: sessionID})
	if err != nil {
		return err
	}
	return e.svc.AppendEvent(ctx, resp.Session, &session.Event{
		Timestamp: time.Now(),
		Author:    author,
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText(content, genai.Role(author)),
		},
	})
}

func (e *episodicMemoryImpl) GetRecentHistory(ctx context.Context, sessionID string, limit int) ([]string, error) {
	var history []string
	resp, err := e.svc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: e.userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}

	// Collect all events then slice
	var allEvents []string
	for ev := range resp.Session.Events().All() {
		if ev.Content != nil {
			for _, part := range ev.Content.Parts {
				if part.Text != "" {
					text := part.Text
					if len(text) > 500 {
						text = text[:500] + "..."
					}
					allEvents = append(allEvents, fmt.Sprintf("[%s]: %s", ev.Author, text))
				}
			}
		}
	}

	if len(allEvents) > limit {
		history = allEvents[len(allEvents)-limit:]
	} else {
		history = allEvents
	}

	return history, nil
}

func (e *episodicMemoryImpl) EpisodicStats(ctx context.Context, sessionID string) MemoryStats {
	resp, err := e.svc.Get(ctx, &session.GetRequest{AppName: "swarm-cli", UserID: e.userID, SessionID: sessionID})
	if err != nil {
		return MemoryStats{Name: "Episodic Memory (Tier 2)"}
	}

	count := 0
	tokens := 0
	for ev := range resp.Session.Events().All() {
		count++
		if ev.Content != nil {
			for _, part := range ev.Content.Parts {
				tokens += len(part.Text) / 4
			}
		}
	}

	return MemoryStats{
		Name:          "Episodic Memory (Tier 2)",
		Count:         count,
		TokenEstimate: tokens,
	}
}

// defaultHierarchicalMemory implements the HierarchicalMemory interface.
type defaultHierarchicalMemory struct {
	working  WorkingMemory
	episodic EpisodicMemory
	semantic SemanticMemory
	global   GlobalMemory
}

func NewHierarchicalMemory(w WorkingMemory, e EpisodicMemory, s SemanticMemory, g GlobalMemory) HierarchicalMemory {
	return &defaultHierarchicalMemory{
		working:  w,
		episodic: e,
		semantic: s,
		global:   g,
	}
}

func (m *defaultHierarchicalMemory) Working() WorkingMemory   { return m.working }
func (m *defaultHierarchicalMemory) Episodic() EpisodicMemory { return m.episodic }
func (m *defaultHierarchicalMemory) Semantic() SemanticMemory { return m.semantic }
func (m *defaultHierarchicalMemory) Global() GlobalMemory     { return m.global }
