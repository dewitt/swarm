package sdk_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dewitt/swarm/pkg/sdk"
)

// Mocks and helpers are in mock_model_test.go

func TestNewSwarm(t *testing.T) {
	mockLLM := &sdk.MockModel{Response: "Hello from the mock agent!"}
	swarm, err := sdk.NewSwarm(sdk.SwarmConfig{Model: mockLLM})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if swarm == nil {
		t.Fatal("expected swarm to be non-nil")
	}

	ctx := context.Background()

	ch, err := swarm.Chat(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var finalResponses []string
	for event := range ch {
		if event.State == sdk.AgentStateComplete && event.FinalContent != "" {
			finalResponses = append(finalResponses, event.FinalContent)
		}
	}

	if len(finalResponses) == 0 {
		t.Fatalf("expected at least one final Response")
	}

	expected := "Hello from trivial plan!"
	found := false
	for _, r := range finalResponses {
		if strings.Contains(r, expected) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected content '%s' in Responses, got %v", expected, finalResponses)
	}
}

func TestChat_TrivialResponse(t *testing.T) {
	mockLLM := &sdk.MockModel{Response: "Trivial Response"}
	swarm, err := sdk.NewSwarm(sdk.SwarmConfig{Model: mockLLM})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ctx := context.Background()
	// Trivial prompt that should trigger ImmediateResponse in our MockModel's Planning mode
	ch, err := swarm.Chat(ctx, "Hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var events []sdk.ObservableEvent
	for event := range ch {
		events = append(events, event)
	}

	foundFinal := false
	for _, e := range events {
		if e.State == sdk.AgentStateComplete && e.FinalContent != "" {
			foundFinal = true
		}
	}

	if !foundFinal {
		t.Fatalf("expected a final Response event for trivial query")
	}
}
