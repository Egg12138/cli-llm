package main

import (
	"context"
	"testing"
)

func TestChatModelAgentExample(t *testing.T) {
	t.Parallel()

	events, err := RunDemo(context.Background(), "Ada")
	if err != nil {
		t.Fatalf("RunDemo returned error: %v", err)
	}

	expected := []DemoEvent{
		{Kind: "model", Content: "calling greet"},
		{Kind: "tool", ToolName: "greet", Content: "hello Ada"},
		{Kind: "model", Content: "tool said hello Ada"},
	}
	if len(events) != len(expected) {
		t.Fatalf("expected events %#v, got %#v", expected, events)
	}
	for i := range expected {
		if events[i] != expected[i] {
			t.Fatalf("expected events %#v, got %#v", expected, events)
		}
	}
}
