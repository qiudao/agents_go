package main

import "context"

// Provider abstracts the LLM API so the agent loop is provider-agnostic.
type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
}

// Message represents a conversation turn.
type Message struct {
	Role    string         // "user", "assistant"
	Content []ContentBlock
}

// ContentBlock is a single piece of content within a message.
type ContentBlock struct {
	Type     string         // "text", "tool_use", "tool_result"
	Text     string
	ToolID   string
	ToolName string
	Input    map[string]any
}

// Tool describes a function the model can call.
type Tool struct {
	Name        string
	Description string
	Properties  map[string]any
}

// Response is what the provider returns from a Chat call.
type Response struct {
	WantsTool bool           // true = model wants to call a tool
	Content   []ContentBlock
}
