package sdk_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dewitt/swarm/pkg/sdk"
)

func TestSemanticMemoryE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	if os.Getenv("SWARM_RUN_E2E") == "" {
		t.Skip("Skipping live E2E test; set SWARM_RUN_E2E=1 to run")
	}

	if os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("skipping E2E test because GOOGLE_API_KEY is not set")
	}

	// 1. Setup workspace
	tmpDir, err := os.MkdirTemp("", "swarm-e2e-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	secretsDir := filepath.Join(tmpDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("failed to create secrets dir: %v", err)
	}

	secretFile := filepath.Join(secretsDir, "secret_recipe.txt")
	secretContent := "THE MAGICAL HIDDEN INGREDIENT IS 'STRAWBERRY_PIE'"
	if err := os.WriteFile(secretFile, []byte(secretContent), 0o600); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	// Change working directory to the temp dir so Swarm considers it the project root
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	projectRoot := filepath.Join(originalWd, "..", "..")
	skillsDir := filepath.Join(projectRoot, "skills")

	err = os.Symlink(skillsDir, filepath.Join(tmpDir, "skills"))
	if err != nil {
		t.Fatalf("failed to symlink skills dir: %v", err)
	}

	err = os.Symlink(filepath.Join(projectRoot, "AGENTS.md"), filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to symlink AGENTS.md: %v", err)
	}

	// 2. Initialize Swarm (using the real Gemini model)
	swarm, err := sdk.NewSwarm(sdk.SwarmConfig{
		DatabaseURI: "file::memory:?cache=shared",
	})
	if err != nil {
		t.Fatalf("failed to init swarm: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 3. First Query: Instruct the agent to find the ingredient.
	// We do NOT tell it to commit anything; we expect passive extraction.
	t.Log("Sending Query 1: Discovering the fact (Passive)...")
	q1 := "There is a secret recipe located in the secrets directory. Find it and tell me the hidden ingredient in a clear, natural sentence."
	ch1, err := swarm.Chat(ctx, q1)
	if err != nil {
		t.Fatalf("failed to start chat 1: %v", err)
	}

	var finalResponse1 []string
	for event := range ch1 {
		if event.State == sdk.AgentStateError {
			t.Fatalf("Swarm encountered error in query 1. Error: %v, FinalContent: %v", event.Error, event.FinalContent)
		}
		if event.State == sdk.AgentStateExecuting && event.ToolName != "" {
			t.Logf("Query 1 Tool Used: %s", event.ToolName)
			if event.ToolName == "commit_fact" {
				t.Fatalf("Agent used commit_fact! It should have been removed and we expect passive extraction.")
			}
		}
		if event.State == sdk.AgentStateComplete && event.FinalContent != "" && event.AgentName != "Observer" {
			finalResponse1 = append(finalResponse1, event.FinalContent)
		}
	}

	joinedResponse1 := strings.Join(finalResponse1, " ")
	t.Logf("Query 1 Final Response: %s", joinedResponse1)
	if !strings.Contains(joinedResponse1, "STRAWBERRY_PIE") {
		t.Errorf("Query 1 failed to return the ingredient. Responses: %v", finalResponse1)
	}

	// Verify that the fact was AUTOMATICALLY committed to the semantic memory database via reflection
	facts, err := swarm.ListFacts(10)
	if err != nil {
		t.Fatalf("failed to list facts: %v", err)
	}
	t.Logf("Facts in memory after Q1: %v", facts)
	factFound := false
	for _, f := range facts {
		if strings.Contains(f, "STRAWBERRY_PIE") {
			factFound = true
			break
		}
	}
	if !factFound {
		t.Fatalf("Passive extraction failed! The fact was not found in semantic memory after Query 1. Facts: %v", facts)
	}
	t.Log("Passive extraction successful. Fact committed to semantic memory.")

	// 4. Second Query: Ask for the ingredient directly.
	// This should hit the semantic memory injected into the prompt and NOT require searching the filesystem.
	t.Log("Sending Query 2: Retrieving the fact efficiently...")
	q2 := "What is the hidden ingredient? Answer quickly based on your memory."
	ch2, err := swarm.Chat(ctx, q2)
	if err != nil {
		t.Fatalf("failed to start chat 2: %v", err)
	}

	var finalResponse2 []string
	toolsUsed := 0

	for event := range ch2 {
		if event.State == sdk.AgentStateError {
			t.Fatalf("Swarm encountered error in query 2. Error: %v, FinalContent: %v", event.Error, event.FinalContent)
		}
		if event.State == sdk.AgentStateExecuting && event.ToolName != "" {
			t.Logf("Query 2 Tool Used: %s", event.ToolName)
			// It should NOT be reading files or listing directories.
			if event.ToolName == "read_local_file" || event.ToolName == "grep_search" || event.ToolName == "list_local_files" || event.ToolName == "bash_execute" {
				toolsUsed++
			}
		}
		if event.State == sdk.AgentStateComplete && event.FinalContent != "" && event.AgentName != "Observer" {
			finalResponse2 = append(finalResponse2, event.FinalContent)
		}
	}

	joinedResponse2 := strings.Join(finalResponse2, " ")
	if !strings.Contains(joinedResponse2, "STRAWBERRY_PIE") {
		t.Errorf("Query 2 failed to retrieve the ingredient from memory. Responses: %v", finalResponse2)
	}

	if toolsUsed > 0 {
		t.Errorf("Query 2 was inefficient! It used %d file system/search tools instead of relying on semantic memory.", toolsUsed)
	} else {
		t.Log("Query 2 successfully utilized semantic memory without executing file system tools.")
	}
}
