package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	compactThreshold = 50000 // input tokens
	keepRecent       = 3     // tool results to preserve
)

// microCompact replaces old tool_result content with placeholders.
// Keeps the most recent keepRecent results intact.
func microCompact(messages []Message) {
	type toolResultRef struct {
		msgIdx   int
		blockIdx int
	}
	var refs []toolResultRef

	for i, m := range messages {
		if m.Role != "user" {
			continue
		}
		for j, b := range m.Content {
			if b.Type == "tool_result" {
				refs = append(refs, toolResultRef{i, j})
			}
		}
	}

	if len(refs) <= keepRecent {
		return
	}

	toClear := refs[:len(refs)-keepRecent]
	for _, ref := range toClear {
		b := &messages[ref.msgIdx].Content[ref.blockIdx]
		if len(b.Text) > 100 {
			toolName := b.ToolName
			if toolName == "" {
				toolName = "unknown"
			}
			b.Text = fmt.Sprintf("[Previous: used %s]", toolName)
		}
	}
}

// saveTranscript writes the full message history to a JSONL file.
func saveTranscript(dir string, messages []Message) string {
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, fmt.Sprintf("transcript_%d.jsonl", time.Now().Unix()))
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, m := range messages {
		enc.Encode(m)
	}
	return path
}

// transcriptDir returns the path for storing transcripts.
func transcriptDir() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".transcripts")
}

// buildCompactedMessages replaces all messages with a compressed summary.
func buildCompactedMessages(summary, transcriptPath string) []Message {
	return []Message{
		{Role: "user", Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("[Conversation compressed. Transcript: %s]\n\n%s", transcriptPath, summary),
		}}},
		{Role: "assistant", Content: []ContentBlock{{
			Type: "text",
			Text: "Understood. I have the context from the summary. Continuing.",
		}}},
	}
}

// summarizeForCompact asks the LLM to summarize the conversation.
func summarizeForCompact(provider Provider, messages []Message, model string) string {
	text, _ := json.Marshal(messages)
	if len(text) > 80000 {
		text = text[:80000]
	}

	prompt := "Summarize this conversation for continuity. Include: " +
		"1) What was accomplished, 2) Current state, 3) Key decisions made. " +
		"Be concise but preserve critical details.\n\n" + string(text)

	summaryMsgs := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: prompt}}},
	}

	resp, err := provider.Chat(context.Background(), summaryMsgs, nil)
	if err != nil {
		return "(summary failed: " + err.Error() + ")"
	}
	for _, b := range resp.Content {
		if b.Type == "text" {
			return b.Text
		}
	}
	return "(no summary produced)"
}
