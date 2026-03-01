package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider using the Google GenAI SDK.
// Supports multiple API keys with automatic rotation on rate limit errors.
type GeminiProvider struct {
	clients []*genai.Client
	current int
	model   string
}

// NewGeminiProvider creates a provider. apiKeys can be comma-separated for rotation.
func NewGeminiProvider(ctx context.Context, apiKeys, model string) (*GeminiProvider, error) {
	keys := strings.Split(apiKeys, ",")
	var clients []*genai.Client
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		c, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  k,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("gemini client: %w", err)
		}
		clients = append(clients, c)
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid GEMINI_API_KEY provided")
	}
	fmt.Printf("Gemini: %d API key(s) loaded\n", len(clients))
	return &GeminiProvider{clients: clients, model: model}, nil
}

func (p *GeminiProvider) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	contents := toGeminiContents(messages)
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{toGeminiTool(tools)},
	}

	var lastErr error
	for range p.clients {
		resp, err := p.clients[p.current].Models.GenerateContent(ctx, p.model, contents, config)
		if err != nil {
			if isRateLimitError(err) && len(p.clients) > 1 {
				prev := p.current
				p.current = (p.current + 1) % len(p.clients)
				fmt.Printf("\033[33mKey #%d rate limited, switching to key #%d\033[0m\n", prev+1, p.current+1)
				lastErr = err
				continue
			}
			return nil, err
		}
		return fromGeminiResponse(resp), nil
	}
	return nil, fmt.Errorf("all %d keys rate limited: %w", len(p.clients), lastErr)
}

func isRateLimitError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "RESOURCE_EXHAUSTED") || strings.Contains(s, "rate")
}

func toGeminiContents(msgs []Message) []*genai.Content {
	var out []*genai.Content
	for _, m := range msgs {
		c := &genai.Content{Role: toGeminiRole(m.Role)}
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				c.Parts = append(c.Parts, &genai.Part{Text: b.Text})
			case "tool_use":
				c.Parts = append(c.Parts, genai.NewPartFromFunctionCall(b.ToolName, b.Input))
			case "tool_result":
				c.Parts = append(c.Parts, genai.NewPartFromFunctionResponse(b.ToolName, map[string]any{
					"output": b.Text,
				}))
			}
		}
		out = append(out, c)
	}
	return out
}

func toGeminiRole(role string) string {
	if role == "assistant" {
		return "model"
	}
	return role
}

func toGeminiTool(tools []Tool) *genai.Tool {
	var decls []*genai.FunctionDeclaration
	for _, t := range tools {
		props := make(map[string]*genai.Schema)
		for k, v := range t.Properties {
			if m, ok := v.(map[string]any); ok {
				s := &genai.Schema{Type: genai.Type(strings.ToUpper(fmt.Sprintf("%v", m["type"])))}
				if desc, ok := m["description"].(string); ok {
					s.Description = desc
				}
				props[k] = s
			}
		}
		decls = append(decls, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: props,
			},
		})
	}
	return &genai.Tool{FunctionDeclarations: decls}
}

func fromGeminiResponse(resp *genai.GenerateContentResponse) *Response {
	r := &Response{}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return r
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			r.WantsTool = true
			r.Content = append(r.Content, ContentBlock{
				Type:     "tool_use",
				ToolID:   part.FunctionCall.ID,
				ToolName: part.FunctionCall.Name,
				Input:    part.FunctionCall.Args,
			})
		} else if part.Text != "" {
			r.Content = append(r.Content, ContentBlock{Type: "text", Text: part.Text})
		}
	}

	return r
}
