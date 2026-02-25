package sdk_test

import (
	"context"
	"testing"

	"github.com/dewitt/agents/pkg/sdk"
)

func TestNewManager(t *testing.T) {
	manager := sdk.NewManager()
	if manager == nil {
		t.Fatal("expected manager to be non-nil")
	}

	ctx := context.Background()
	
	// Test the Chat stub to ensure the channels don't block
	ch, err := manager.Chat(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := <-ch
	if response == "" {
		t.Fatal("expected a response from the Chat method")
	}
}
