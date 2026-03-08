package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticRetrievalInjection(t *testing.T) {
	// 1. Setup workspace
	tmpDir, _ := os.MkdirTemp("", "swarm-memory-retrieval-*")
	defer os.RemoveAll(tmpDir)

	skillsDir := filepath.Join(tmpDir, "skills", "swarm-agent")
	_ = os.MkdirAll(skillsDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("---\nname: swarm_agent\n---\nHello"), 0o600)

	originalWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(originalWd) }()

	// 2. Initialize Swarm
	swarm, err := NewSwarm(SwarmConfig{
		DatabaseURI: "file::memory:?cache=shared",
		Model:       &MockModel{Response: "{}"},
	})
	if err != nil {
		t.Fatalf("failed to init swarm: %v", err)
	}

	// 3. Manually commit a fact
	fact := "PROJECT_FACT: The secret code is 'BLUE-SQUIRREL'."
	err = swarm.(*defaultSwarm).memory.Semantic().Commit(fact)
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// 4. Test logic inside Plan()
	m := swarm.(*defaultSwarm)
	prompt := "What is the secret code?"

	// Duplicate the logic from Plan to see if facts are found
	cleanPrompt := strings.ReplaceAll(prompt, "?", "")
	cleanPrompt = strings.ReplaceAll(cleanPrompt, "\"", "")
	cleanPrompt = strings.ReplaceAll(cleanPrompt, "'", "")

	userFacts, err := m.memory.Semantic().Retrieve(cleanPrompt, 3)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}

	if len(userFacts) == 0 {
		t.Errorf("FAIL: No facts retrieved for prompt: %s", cleanPrompt)
	} else {
		fmt.Printf("SUCCESS: Retrieved %d facts: %v\n", len(userFacts), userFacts)
		if !strings.Contains(userFacts[0], "BLUE-SQUIRREL") {
			t.Errorf("FAIL: Retrieved wrong fact: %s", userFacts[0])
		}
	}
}
