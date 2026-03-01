package main

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements Provider using the Anthropic SDK.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
}

func NewAnthropicProvider(apiKey, baseURL, model string) *AnthropicProvider {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &AnthropicProvider{
		client: anthropic.NewClient(opts...),
		model:  model,
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	anthMsgs := toAnthropicMessages(messages)
	anthTools := toAnthropicTools(tools)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 8000,
		Messages:  anthMsgs,
		Tools:     anthTools,
	})
	if err != nil {
		return nil, err
	}

	return fromAnthropicResponse(resp), nil
}

func toAnthropicMessages(msgs []Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range msgs {
		var blocks []anthropic.ContentBlockParamUnion
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				blocks = append(blocks, anthropic.NewTextBlock(b.Text))
			case "tool_use":
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    b.ToolID,
						Name:  b.ToolName,
						Input: b.Input,
					},
				})
			case "tool_result":
				blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolID, b.Text, false))
			}
		}
		out = append(out, anthropic.MessageParam{
			Role:    anthropic.MessageParamRole(m.Role),
			Content: blocks,
		})
	}
	return out
}

func toAnthropicTools(tools []Tool) []anthropic.ToolUnionParam {
	var out []anthropic.ToolUnionParam
	for _, t := range tools {
		tool := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.Properties,
			},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return out
}

func fromAnthropicResponse(resp *anthropic.Message) *Response {
	r := &Response{
		WantsTool: resp.StopReason == anthropic.StopReason(anthropic.MessageStopReasonToolUse),
	}
	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			r.Content = append(r.Content, ContentBlock{Type: "text", Text: v.Text})
		case anthropic.ToolUseBlock:
			inputMap := make(map[string]any)
			inputJSON, _ := json.Marshal(v.Input)
			_ = json.Unmarshal(inputJSON, &inputMap)
			r.Content = append(r.Content, ContentBlock{
				Type:     "tool_use",
				ToolID:   v.ID,
				ToolName: v.Name,
				Input:    inputMap,
			})
		}
	}
	return r
}
