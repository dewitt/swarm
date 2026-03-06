package eval

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping agentic evaluation in short mode")
	}
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Skip("skipping agentic evaluation; GOOGLE_API_KEY not set")
	}

	evaluator, err := NewEvaluator(apiKey)
	require.NoError(t, err)

	scenarios, err := GetScenarios()
	require.NoError(t, err)

	for _, scenario := range scenarios {
		t.Run(scenario.ID, func(t *testing.T) {
			result, err := evaluator.Run(context.Background(), scenario)
			require.NoError(t, err)

			if result.Passed {
				t.Logf("Evaluator Passed: %s", result.Reasoning)
			} else {
				t.Fatalf("Evaluator Failed: %s\nTrajectory:\n%s", result.Reasoning, result.Trajectory)
			}
			assert.True(t, result.Passed)
		})
	}
}
