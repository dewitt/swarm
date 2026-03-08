package sdk

import (
	"os"
	"testing"
)

func TestSemanticMemory(t *testing.T) {
	// Create a temporary workspace for testing
	tmpDir, err := os.MkdirTemp("", "swarm-semantic-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mem, err := NewSemanticMemory(tmpDir)
	if err != nil {
		t.Fatalf("failed to initialize semantic memory: %v", err)
	}

	// 1. Test Commit
	fact1 := "The largest Go file is interactive.go"
	fact2 := "TOOL FAILURE OFFLINE: codex is out of credits"

	if err := mem.Commit(fact1); err != nil {
		t.Fatalf("failed to commit fact1: %v", err)
	}
	if err := mem.Commit(fact2); err != nil {
		t.Fatalf("failed to commit fact2: %v", err)
	}

	// 2. Test List
	list, err := mem.List(10)
	if err != nil {
		t.Fatalf("failed to list facts: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 facts in list, got %d", len(list))
	}
	// order should be descending (newest first)
	if list[0] != fact2 {
		t.Errorf("expected newest fact first, got %s", list[0])
	}

	// 3. Test Retrieve (FTS or LIKE fallback)
	results, err := mem.Retrieve("interactive.go", 5)
	if err != nil {
		t.Fatalf("failed to retrieve fact: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'interactive.go', got %d", len(results))
	}
	if results[0] != fact1 {
		t.Errorf("expected '%s', got '%s'", fact1, results[0])
	}

	// Test punctuation handling in queries
	results, err = mem.Retrieve("TOOL FAILURE OFFLINE", 5)
	if err != nil {
		t.Fatalf("failed to retrieve tool failure: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'TOOL FAILURE', got %d", len(results))
	}
	if results[0] != fact2 {
		t.Errorf("expected '%s', got '%s'", fact2, results[0])
	}
}
