package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger writes conversation rounds to a JSONL file. Always on, always full.
type Logger struct {
	file  *os.File
	round int
}

// LogEntry is one line in the JSONL log.
type LogEntry struct {
	Round     int    `json:"round"`
	Timestamp string `json:"ts"`
	Type      string `json:"type"` // "request", "response"

	// request
	Model        string `json:"model,omitempty"`
	MaxTokens    int    `json:"max_tokens,omitempty"`
	FullMessages []any  `json:"full_messages,omitempty"`
	Tools        []any  `json:"tools,omitempty"`

	// response
	Content    []any  `json:"content,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Reminder   bool   `json:"reminder,omitempty"`
}

func newSessionLogger() (*Logger, error) {
	dir := filepath.Join(configDir(), "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	name := "session_" + time.Now().Format("2006-01-02_150405") + ".jsonl"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

func (l *Logger) Path() string {
	return l.file.Name()
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) NextRound() {
	l.round++
}

func (l *Logger) write(entry LogEntry) {
	entry.Round = l.round
	entry.Timestamp = time.Now().Format(time.RFC3339)
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	l.file.Write(data)
	l.file.Write([]byte("\n"))
}

// messagesToAny converts internal Messages to generic maps for logging.
func messagesToAny(msgs []Message) []any {
	var out []any
	for _, m := range msgs {
		msg := map[string]any{"role": m.Role}
		var content []any
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				content = append(content, map[string]any{
					"type": "text",
					"text": b.Text,
				})
			case "tool_use":
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    b.ToolID,
					"name":  b.ToolName,
					"input": b.Input,
				})
			case "tool_result":
				content = append(content, map[string]any{
					"type":        "tool_result",
					"tool_use_id": b.ToolID,
					"name":        b.ToolName,
					"content":     b.Text,
				})
			}
		}
		msg["content"] = content
		out = append(out, msg)
	}
	return out
}

// toolsToAny converts Tool slice to generic maps for logging.
func toolsToAny(tls []Tool) []any {
	var out []any
	for _, t := range tls {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"input_schema": map[string]any{
				"type":       "object",
				"properties": t.Properties,
			},
		})
	}
	return out
}

// LogRequest logs the full request sent to the model.
func (l *Logger) LogRequest(messages []Message, tls []Tool, model string) {
	l.write(LogEntry{
		Type:         "request",
		Model:        model,
		MaxTokens:    8000,
		FullMessages: messagesToAny(messages),
		Tools:        toolsToAny(tls),
	})
}

// LogResponse logs the model's response.
func (l *Logger) LogResponse(content []ContentBlock, stopReason string, hasReminder bool) {
	var blocks []any
	for _, b := range content {
		switch b.Type {
		case "text":
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case "tool_use":
			blocks = append(blocks, map[string]any{
				"type":  "tool_use",
				"id":    b.ToolID,
				"name":  b.ToolName,
				"input": b.Input,
			})
		}
	}
	l.write(LogEntry{
		Type:       "response",
		Content:    blocks,
		StopReason: stopReason,
		Reminder:   hasReminder,
	})
}

// showLog reads the JSONL log file and prints raw communication.
func showLog(path string) {
	if sessionLogger != nil && sessionLogger.file != nil {
		sessionLogger.file.Sync()
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var raw map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}
		typ, _ := raw["type"].(string)
		round, _ := raw["round"].(float64)

		switch typ {
		case "request":
			fmt.Printf("\n\033[1;36m========== Round %.0f: Agent → Model ==========\033[0m\n", round)
			pretty, _ := json.MarshalIndent(raw, "", "  ")
			fmt.Println(string(pretty))
		case "response":
			fmt.Printf("\n\033[1;33m========== Round %.0f: Model → Agent ==========\033[0m\n", round)
			pretty, _ := json.MarshalIndent(raw, "", "  ")
			fmt.Println(string(pretty))
		}
	}
	fmt.Println()
}
