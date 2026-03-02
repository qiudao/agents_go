// s01_agent_loop.go - The Agent Loop (Go edition)
//
// The entire secret of an AI coding agent in one pattern:
//
//	while stop_reason == "tool_use":
//	    response = LLM(messages, tools)
//	    execute tools
//	    append results
//
//	+----------+      +-------+      +---------+
//	|   User   | ---> |  LLM  | ---> |  Tool   |
//	|  prompt  |      |       |      | execute |
//	+----------+      +---+---+      +----+----+
//	                      ^               |
//	                      |   tool_result |
//	                      +---------------+
//	                      (loop continues)

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/joho/godotenv"
)

func runBash(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "Error: Timeout (120s)"
	}

	out := strings.TrimSpace(string(output))
	if err != nil && out == "" {
		return fmt.Sprintf("Error: %v", err)
	}
	if out == "" {
		return "(no output)"
	}
	if len(out) > 50000 {
		return out[:50000]
	}
	return out
}

// executeTool dispatches a tool call and returns its output.
func executeTool(b ContentBlock) string {
	switch b.ToolName {
	case "bash":
		command, _ := b.Input["command"].(string)
		fmt.Printf("\033[33m$ %s\033[0m\n", command)
		if !isCommandAllowed(command, loadAllowedCommands()) {
			fmt.Printf("Allow? [y]es / [n]o / [a]lways: ")
			var answer string
			fmt.Scanln(&answer)
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "n", "no":
				return "Command denied by user."
			case "a", "always":
				prefix := commandPrefix(command)
				addAllowedCommand(prefix)
				fmt.Printf("Allowed prefix \"%s\" saved to %s\n", prefix, configPath())
			}
		}
		return runBash(command)
	case "web_search":
		query, _ := b.Input["query"].(string)
		fmt.Printf("\033[33m🔍 %s\033[0m\n", query)
		return webSearch(query)
	case "web_fetch":
		fetchURL, _ := b.Input["url"].(string)
		prompt, _ := b.Input["prompt"].(string)
		fmt.Printf("\033[33m🌐 %s\033[0m\n", fetchURL)
		return webFetch(fetchURL, prompt)
	case "read_file":
		path, _ := b.Input["path"].(string)
		limit, _ := b.Input["limit"].(float64)  // JSON numbers are float64
		offset, _ := b.Input["offset"].(float64) // JSON numbers are float64
		fmt.Printf("\033[33m📖 %s\033[0m\n", path)
		return runRead(path, int(limit), int(offset))
	case "write_file":
		path, _ := b.Input["path"].(string)
		content, _ := b.Input["content"].(string)
		fmt.Printf("\033[33m✏️ %s\033[0m\n", path)
		return runWrite(path, content)
	case "edit_file":
		path, _ := b.Input["path"].(string)
		oldText, _ := b.Input["old_text"].(string)
		newText, _ := b.Input["new_text"].(string)
		fmt.Printf("\033[33m✏️ %s\033[0m\n", path)
		return runEdit(path, oldText, newText)
	default:
		return fmt.Sprintf("Unknown tool: %s", b.ToolName)
	}
}

// tools available to the agent.
var tools = []Tool{
	{
		Name:        "bash",
		Description: "Run a shell command.",
		Properties: map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to run",
			},
		},
	},
	{
		Name:        "web_search",
		Description: "Search the web using Google. Returns titles, URLs, and snippets for the top results.",
		Properties: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
	},
	{
		Name:        "read_file",
		Description: "Read file contents with line numbers. Rejects binary files and files >10MB.",
		Properties: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path (relative to working directory)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max lines to read (optional, 0 = all)",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Number of lines to skip from the start (optional, 0 = none)",
			},
		},
	},
	{
		Name:        "write_file",
		Description: "Write content to a file (creates parent dirs).",
		Properties: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path (relative to working directory)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write",
			},
		},
	},
	{
		Name:        "edit_file",
		Description: "Replace exact text in a file. old_text must match exactly once; include more surrounding context if it matches multiple locations.",
		Properties: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path (relative to working directory)",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "Exact text to find (must be unique in the file)",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "Replacement text",
			},
		},
	},
	{
		Name:        "web_fetch",
		Description: "Fetch a web page and extract key information. A small model preprocesses the content to return only relevant information.",
		Properties: map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "What information to extract from the page (optional, defaults to general summary)",
			},
		},
	},
}

// agentLoop is the core pattern: call LLM with tools, execute tool calls,
// feed results back, repeat until the model stops.
func agentLoop(provider Provider, messages *[]Message) error {
	for {
		resp, err := provider.Chat(context.TODO(), *messages, tools)
		if err != nil {
			return fmt.Errorf("API error: %w", err)
		}

		// Append assistant turn
		*messages = append(*messages, Message{Role: "assistant", Content: resp.Content})

		// If the model didn't call a tool, print text and we're done
		if !resp.WantsTool {
			for _, b := range resp.Content {
				if b.Type == "text" {
					fmt.Println(b.Text)
				}
			}
			return nil
		}

		// Execute each tool call, collect results
		var results []ContentBlock
		for _, b := range resp.Content {
			switch b.Type {
			case "text":
				fmt.Println(b.Text)
			case "tool_use":
				output := executeTool(b)
				if len(output) > 200 {
					fmt.Println(output[:200])
				} else {
					fmt.Println(output)
				}
				results = append(results, ContentBlock{
					Type:     "tool_result",
					ToolID:   b.ToolID,
					ToolName: b.ToolName,
					Text:     output,
				})
			}
		}
		*messages = append(*messages, Message{Role: "user", Content: results})
	}
}

func newProvider(prov, model string) (Provider, error) {
	switch prov {
	case "anthropic":
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		return NewAnthropicProvider(
			os.Getenv("ANTHROPIC_API_KEY"),
			os.Getenv("ANTHROPIC_BASE_URL"),
			model,
		), nil

	case "deepseek":
		if model == "" {
			model = "deepseek-chat"
		}
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			cfg := loadConfig()
			apiKey = cfg["DEEPSEEK_API_KEY"]
		}
		if apiKey == "" {
			return nil, fmt.Errorf("DEEPSEEK_API_KEY is required (set in env, .env, or %s)", configPath())
		}
		return NewAnthropicProvider(apiKey, "https://api.deepseek.com/anthropic", model), nil

	case "gemini":
		if model == "" {
			model = "gemini-2.5-flash"
		}
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			cfg := loadConfig()
			apiKey = cfg["GEMINI_API_KEY"]
		}
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is required (set in env, .env, or %s)", configPath())
		}
		return NewGeminiProvider(context.Background(), apiKey, model)

	case "qwen":
		if model == "" {
			model = "qwen-plus"
		}
		apiKey := os.Getenv("QWEN_API_KEY")
		if apiKey == "" {
			cfg := loadConfig()
			apiKey = cfg["QWEN_API_KEY"]
		}
		if apiKey == "" {
			return nil, fmt.Errorf("QWEN_API_KEY is required (set in env, .env, or %s)", configPath())
		}
		return NewOpenAIProvider(apiKey, "https://dashscope.aliyuncs.com/compatible-mode/v1", model), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s (use anthropic, gemini, deepseek, or qwen)", prov)
	}
}

// resolveProviderModel determines provider+model from: env > config > defaults.
func resolveProviderModel() (string, string) {
	prov := os.Getenv("PROVIDER")
	model := os.Getenv("MODEL_ID")
	if model == "" {
		model = os.Getenv("GEMINI_MODEL")
	}

	if prov == "" {
		cfg := loadConfig()
		if p, ok := cfg["PROVIDER"]; ok {
			prov = p
		}
		if model == "" {
			if m, ok := cfg["MODEL"]; ok {
				model = m
			}
		}
	}

	if prov == "" {
		// auto-detect: pick whichever has a key
		if os.Getenv("GEMINI_API_KEY") != "" {
			prov = "gemini"
		} else {
			prov = "anthropic"
		}
	}
	return prov, model
}

func main() {
	godotenv.Load()
	godotenv.Load("../.env")

	prov, model := resolveProviderModel()
	provider, err := newProvider(prov, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Hint: run the program and type /models to configure\n")
		os.Exit(1)
	}
	cwd, _ := os.Getwd()
	system := fmt.Sprintf("You are a coding agent at %s. Use bash to solve tasks. Act, don't explain.", cwd)

	var messages []Message
	messages = append(messages, Message{
		Role:    "user",
		Content: []ContentBlock{{Type: "text", Text: "[System] " + system}},
	})
	messages = append(messages, Message{
		Role:    "assistant",
		Content: []ContentBlock{{Type: "text", Text: "Understood. I'll use bash to help you."}},
	})



	rl, err := readline.New("\033[36ms01 >> \033[0m")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err == io.EOF || err == readline.ErrInterrupt {
			break
		}
		query := strings.TrimSpace(line)
		if query == "" {
			continue
		}
		if query == "q" || query == "exit" {
			break
		}

		// Handle slash commands
		if query == "/usage" {
			showUsage(prov)
			continue
		}
		if query == "/models" {
			choice := selectModel(rl)
			if choice == nil {
				continue
			}
			cfg := loadConfig()
			cfg["PROVIDER"] = choice.Provider
			cfg["MODEL"] = choice.Model
			if err := saveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				continue
			}
			newProv, err := newProvider(choice.Provider, choice.Model)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}
			provider = newProv
			prov, model = choice.Provider, choice.Model
			fmt.Printf("Switched to: %s/%s (saved to %s)\n", prov, model, configPath())
		
			messages = messages[:2]
			continue
		}

		messages = append(messages, Message{
			Role:    "user",
			Content: []ContentBlock{{Type: "text", Text: query}},
		})
		if err := agentLoop(provider, &messages); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	
	}
}
