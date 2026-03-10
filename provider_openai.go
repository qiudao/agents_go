package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider implements Provider using the OpenAI Chat Completions API.
// Works with any OpenAI-compatible service (Qwen/百炼, OpenAI, etc.).
type OpenAIProvider struct {
	apiKey  string
	baseURL string // e.g. "https://dashscope.aliyuncs.com/compatible-mode/v1"
	model   string
}

func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	return &OpenAIProvider{apiKey: apiKey, baseURL: baseURL, model: model}
}

// ── OpenAI request/response types ────────────────────────────────────────

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type oaiTool struct {
	Type     string          `json:"type"`
	Function oaiToolFunction `json:"function"`
}

type oaiToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message      oaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── Provider implementation ──────────────────────────────────────────────

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	oaiMsgs := toOpenAIMessages(messages)
	oaiTools := toOpenAITools(tools)

	reqBody := oaiRequest{
		Model:    p.model,
		Messages: oaiMsgs,
	}
	if len(oaiTools) > 0 {
		reqBody.Tools = oaiTools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var oaiResp oaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if oaiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", oaiResp.Error.Message)
	}
	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return fromOpenAIResponse(oaiResp), nil
}

// ── Conversion helpers ───────────────────────────────────────────────────

func toOpenAIMessages(msgs []Message) []oaiMessage {
	var out []oaiMessage
	for _, m := range msgs {
		// Check if this message contains tool_use (assistant with tool calls)
		var toolCalls []oaiToolCall
		var textParts []string

		for _, b := range m.Content {
			switch b.Type {
			case "text":
				textParts = append(textParts, b.Text)
			case "tool_use":
				args, _ := json.Marshal(b.Input)
				toolCalls = append(toolCalls, oaiToolCall{
					ID:   b.ToolID,
					Type: "function",
					Function: oaiFunction{
						Name:      b.ToolName,
						Arguments: string(args),
					},
				})
			case "tool_result":
				// Each tool_result becomes a separate "tool" role message
				out = append(out, oaiMessage{
					Role:       "tool",
					Content:    b.Text,
					ToolCallID: b.ToolID,
				})
			}
		}

		// If we have tool calls, emit assistant message with tool_calls
		if len(toolCalls) > 0 {
			msg := oaiMessage{
				Role:      "assistant",
				ToolCalls: toolCalls,
			}
			if len(textParts) > 0 {
				joined := ""
				for _, t := range textParts {
					if joined != "" {
						joined += "\n"
					}
					joined += t
				}
				msg.Content = joined
			}
			out = append(out, msg)
		} else if len(textParts) > 0 {
			// Regular text message
			joined := ""
			for _, t := range textParts {
				if joined != "" {
					joined += "\n"
				}
				joined += t
			}
			out = append(out, oaiMessage{
				Role:    m.Role,
				Content: joined,
			})
		}
	}
	return out
}

func toOpenAITools(tools []Tool) []oaiTool {
	var out []oaiTool
	for _, t := range tools {
		out = append(out, oaiTool{
			Type: "function",
			Function: oaiToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters: map[string]any{
					"type":       "object",
					"properties": t.Properties,
					"required":   propertyKeys(t.Properties),
				},
			},
		})
	}
	return out
}

func propertyKeys(props map[string]any) []string {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	return keys
}

func fromOpenAIResponse(resp oaiResponse) *Response {
	choice := resp.Choices[0]
	r := &Response{
		WantsTool: choice.FinishReason == "tool_calls",
	}

	if resp.Usage != nil {
		r.InputTokens = resp.Usage.PromptTokens
	}

	if choice.Message.Content != "" {
		r.Content = append(r.Content, ContentBlock{
			Type: "text",
			Text: choice.Message.Content,
		})
	}

	for _, tc := range choice.Message.ToolCalls {
		var input map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		r.Content = append(r.Content, ContentBlock{
			Type:     "tool_use",
			ToolID:   tc.ID,
			ToolName: tc.Function.Name,
			Input:    input,
		})
	}

	return r
}
