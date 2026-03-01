package sdk

import (
	"testing"
)

func TestEngine_Initialization(t *testing.T) {
	graph := &ExecutionGraph{
		Spans: []Span{
			{ID: "span1", Name: "Test Span 1"},
		},
	}
	engine := NewEngine(graph)

	if len(engine.spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(engine.spans))
	}
	if engine.status["span1"] != SpanStatusPending {
		t.Errorf("expected span to be pending, got %s", engine.status["span1"])
	}
}

func TestEngine_GetReadySpans(t *testing.T) {
	engine := NewEngine(nil)

	// Add a span with no dependencies
	engine.AddSpans(Span{ID: "s1"})
	// Add a span dependent on s1
	engine.AddSpans(Span{ID: "s2", Dependencies: []string{"s1"}})

	ready := engine.GetReadySpans()
	if len(ready) != 1 || ready[0].ID != "s1" {
		t.Fatalf("expected only s1 to be ready, got %v", ready)
	}

	// Mark s1 as active, it should still not unblock s2
	engine.MarkActive("s1")
	ready = engine.GetReadySpans()
	if len(ready) != 0 {
		t.Fatalf("expected no ready spans while s1 is active, got %v", ready)
	}

	// Mark s1 as complete, s2 should now be ready
	engine.MarkComplete("s1", "result1")
	ready = engine.GetReadySpans()
	if len(ready) != 1 || ready[0].ID != "s2" {
		t.Fatalf("expected s2 to be ready after s1 completes, got %v", ready)
	}
}

func TestEngine_MarkFailed_InvalidatesDependents(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddSpans(
		Span{ID: "s1"},
		// s2 depends on s1
		Span{ID: "s2", Dependencies: []string{"s1"}},
		// s3 depends on s2
		Span{ID: "s3", Dependencies: []string{"s2"}},
		// s4 is independent
		Span{ID: "s4"},
	)

	// Fail s1
	engine.MarkFailed("s1")

	if engine.status["s1"] != SpanStatusFailed {
		t.Errorf("s1 should be failed, got %s", engine.status["s1"])
	}
	if engine.status["s2"] != SpanStatusInvalidated {
		t.Errorf("s2 should be invalidated due to s1 failure, got %s", engine.status["s2"])
	}
	if engine.status["s3"] != SpanStatusInvalidated {
		t.Errorf("s3 should be invalidated due to s2 invalidation, got %s", engine.status["s3"])
	}
	if engine.status["s4"] != SpanStatusPending {
		t.Errorf("s4 should remain pending as it is independent, got %s", engine.status["s4"])
	}
}

func TestEngine_IsComplete(t *testing.T) {
	engine := NewEngine(nil)

	if !engine.IsComplete() {
		t.Error("an empty engine should be complete")
	}

	engine.AddSpans(Span{ID: "s1"})
	if engine.IsComplete() {
		t.Error("engine should not be complete with a pending span")
	}

	engine.MarkActive("s1")
	if engine.IsComplete() {
		t.Error("engine should not be complete with an active span")
	}

	engine.MarkComplete("s1", "done")
	if !engine.IsComplete() {
		t.Error("engine should be complete after span is completed")
	}
}

func TestEngine_GetContext(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddSpans(Span{ID: "s1"}, Span{ID: "s2"})

	engine.MarkComplete("s1", "result for s1")
	engine.MarkComplete("s2", "result for s2")

	ctx := engine.GetContext()
	if ctx["s1"] != "result for s1" {
		t.Errorf("missing or incorrect context for s1: %s", ctx["s1"])
	}
	if ctx["s2"] != "result for s2" {
		t.Errorf("missing or incorrect context for s2: %s", ctx["s2"])
	}
}

func TestEngine_GetTrajectory(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddSpans(Span{ID: "s1"})
	engine.MarkActive("s1")
	engine.MarkComplete("s1", "done")

	traj := engine.GetTrajectory()
	if traj.TraceID == "" {
		t.Errorf("trajectory missing trace ID")
	}
	if len(traj.Spans) != 1 {
		t.Fatalf("expected 1 span in trajectory, got %d", len(traj.Spans))
	}
	if traj.TotalDuration == "" {
		t.Errorf("trajectory missing total duration")
	}
}

func TestEngine_AddSpansWithFailedDependency(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddSpans(Span{ID: "s1"})
	engine.MarkFailed("s1")

	// Adding a new span that depends on a failed one should immediately invalidate it
	engine.AddSpans(Span{ID: "s2", Dependencies: []string{"s1"}})

	if engine.status["s2"] != SpanStatusInvalidated {
		t.Errorf("expected new span with failed dependency to be invalidated, got %s", engine.status["s2"])
	}
}
