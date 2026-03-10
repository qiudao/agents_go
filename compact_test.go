package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMicroCompactKeepsRecent(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
	}
	for i := range 5 {
		toolID := fmt.Sprintf("tool_%d", i)
		toolName := fmt.Sprintf("bash_%d", i)
		messages = append(messages, Message{
			Role: "assistant",
			Content: []ContentBlock{{
				Type: "tool_use", ToolID: toolID, ToolName: toolName,
				Input: map[string]any{"command": "ls"},
			}},
		})
		messages = append(messages, Message{
			Role: "user",
			Content: []ContentBlock{{
				Type: "tool_result", ToolID: toolID, ToolName: toolName,
				Text: strings.Repeat("x", 200),
			}},
		})
	}

	microCompact(messages)

	toolResults := collectToolResults(messages)
	if len(toolResults) != 5 {
		t.Fatalf("expected 5 tool results, got %d", len(toolResults))
	}
	for i, tr := range toolResults {
		if i < 2 {
			if !strings.HasPrefix(tr.Text, "[Previous:") {
				t.Errorf("result %d should be compacted, got: %s", i, tr.Text[:50])
			}
		} else {
			if strings.HasPrefix(tr.Text, "[Previous:") {
				t.Errorf("result %d should be preserved", i)
			}
		}
	}
}

func TestMicroCompactSkipsShortContent(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
	}
	for i := range 5 {
		toolID := fmt.Sprintf("tool_%d", i)
		messages = append(messages, Message{
			Role: "assistant",
			Content: []ContentBlock{{
				Type: "tool_use", ToolID: toolID, ToolName: "bash",
			}},
		})
		messages = append(messages, Message{
			Role: "user",
			Content: []ContentBlock{{
				Type: "tool_result", ToolID: toolID, ToolName: "bash",
				Text: "ok",
			}},
		})
	}

	microCompact(messages)

	for _, tr := range collectToolResults(messages) {
		if strings.HasPrefix(tr.Text, "[Previous:") {
			t.Error("short content should not be compacted")
		}
	}
}

func TestMicroCompactFewResults(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: "assistant", Content: []ContentBlock{{
			Type: "tool_use", ToolID: "t1", ToolName: "bash",
		}}},
		{Role: "user", Content: []ContentBlock{{
			Type: "tool_result", ToolID: "t1", ToolName: "bash",
			Text: strings.Repeat("x", 200),
		}}},
	}

	microCompact(messages)

	tr := collectToolResults(messages)
	if strings.HasPrefix(tr[0].Text, "[Previous:") {
		t.Error("should not compact when <= 3 results")
	}
}

func TestSaveTranscript(t *testing.T) {
	dir := t.TempDir()
	messages := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
	}

	path := saveTranscript(dir, messages)
	if path == "" {
		t.Fatal("expected transcript path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Error("transcript should contain message content")
	}
}

func TestBuildCompactedMessages(t *testing.T) {
	msgs := buildCompactedMessages("summary text", "/tmp/transcript.jsonl")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Error("expected user then assistant")
	}
	if !strings.Contains(msgs[0].Content[0].Text, "summary text") {
		t.Error("user message should contain summary")
	}
	if !strings.Contains(msgs[0].Content[0].Text, "/tmp/transcript.jsonl") {
		t.Error("user message should contain transcript path")
	}
}

func collectToolResults(messages []Message) []ContentBlock {
	var results []ContentBlock
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Type == "tool_result" {
				results = append(results, b)
			}
		}
	}
	return results
}
