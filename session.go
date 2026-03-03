package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

// SessionSnapshot is saved on exit for resume.
type SessionSnapshot struct {
	Timestamp string     `json:"timestamp"`
	Cwd       string     `json:"cwd"`
	Model     string     `json:"model"`
	Messages  []any      `json:"messages"`
	TodoItems []TodoItem `json:"todo_items,omitempty"`
	LogFile   string     `json:"log_file,omitempty"`
	MsgCount  int        `json:"msg_count"`
	Rounds    int        `json:"rounds"`
	Summary   string     `json:"summary"` // first user query
}

func sessionsDir() string {
	return filepath.Join(configDir(), "sessions")
}

// saveSession writes a resume snapshot on exit.
func saveSession(messages []Message, model string, logFile string) error {
	dir := sessionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Don't save if no real conversation happened (only prefill messages)
	if len(messages) <= 2 {
		return nil
	}

	cwd, _ := os.Getwd()
	snap := SessionSnapshot{
		Timestamp: time.Now().Format(time.RFC3339),
		Cwd:       cwd,
		Model:     model,
		Messages:  messagesToAny(messages),
		TodoItems: todo.items,
		LogFile:   logFile,
		MsgCount:  len(messages),
		Rounds:    countRounds(messages),
		Summary:   extractSummary(messages),
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	name := "session_" + time.Now().Format("2006-01-02_150405") + ".json"
	return os.WriteFile(filepath.Join(dir, name), data, 0644)
}

// countRounds counts assistant messages (each = one LLM call).
func countRounds(messages []Message) int {
	n := 0
	for _, m := range messages {
		if m.Role == "assistant" {
			n++
		}
	}
	if n > 0 {
		n-- // subtract the prefill ack message
	}
	return n
}

// extractSummary finds the first real user query (skipping prefill).
func extractSummary(messages []Message) string {
	for i, m := range messages {
		if m.Role == "user" && i >= 2 { // skip prefill (msg 0=system, 1=ack)
			for _, b := range m.Content {
				if b.Type == "text" && b.Text != "" {
					s := b.Text
					if len(s) > 80 {
						s = s[:80] + "..."
					}
					return s
				}
			}
		}
	}
	return ""
}

// SessionInfo is a summary for listing sessions.
type SessionInfo struct {
	Path      string
	Timestamp string
	Cwd       string
	Model     string
	MsgCount  int
	Rounds    int
	Summary   string
}

// listSessions returns available sessions sorted by most recent first.
func listSessions() ([]SessionInfo, error) {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []SessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snap SessionSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		sessions = append(sessions, SessionInfo{
			Path:      path,
			Timestamp: snap.Timestamp,
			Cwd:       snap.Cwd,
			Model:     snap.Model,
			MsgCount:  snap.MsgCount,
			Rounds:    snap.Rounds,
			Summary:   snap.Summary,
		})
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp > sessions[j].Timestamp
	})

	return sessions, nil
}

// resumeSession lets the user pick a session and restores it.
// Returns restored messages, todo items, or nil if cancelled.
func resumeSession(rl *readline.Instance) ([]Message, []TodoItem) {
	sessions, err := listSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil, nil
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions to resume.")
		return nil, nil
	}

	// Show max 10 sessions
	limit := len(sessions)
	if limit > 10 {
		limit = 10
	}

	fmt.Println("Available sessions:")
	cwd, _ := os.Getwd()
	for i, s := range sessions[:limit] {
		t, _ := time.Parse(time.RFC3339, s.Timestamp)
		cwdMark := ""
		if s.Cwd == cwd {
			cwdMark = " ←"
		}
		summary := s.Summary
		if summary == "" {
			summary = "(no query)"
		}
		fmt.Printf("  %d) %s  %s  %d rounds  \"%s\"%s\n",
			i+1, t.Format("2006-01-02 15:04"), s.Model, s.Rounds, summary, cwdMark)
	}

	fmt.Printf("Select [1-%d] or q to cancel: ", limit)
	line, err := rl.Readline()
	if err != nil {
		return nil, nil
	}
	line = strings.TrimSpace(line)
	if line == "q" || line == "" {
		return nil, nil
	}

	var choice int
	if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > limit {
		fmt.Println("Invalid selection.")
		return nil, nil
	}

	selected := sessions[choice-1]
	data, err := os.ReadFile(selected.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil, nil
	}

	var snap SessionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil, nil
	}

	// Convert []any back to []Message
	messages, err := anyToMessages(snap.Messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing messages: %v\n", err)
		return nil, nil
	}

	if selected.Cwd != cwd {
		fmt.Printf("Note: session was in %s (current: %s)\n", selected.Cwd, cwd)
	}
	fmt.Printf("Resumed session (%d rounds, %d messages)\n", snap.Rounds, len(messages))

	return messages, snap.TodoItems
}

// showResumeHint shows a hint about the most recent session in current cwd.
func showResumeHint() {
	cwd, _ := os.Getwd()
	sessions, err := listSessions()
	if err != nil || len(sessions) == 0 {
		return
	}

	for _, s := range sessions {
		if s.Cwd == cwd {
			t, _ := time.Parse(time.RFC3339, s.Timestamp)
			summary := s.Summary
			if summary == "" {
				summary = "(no query)"
			}
			fmt.Printf("Previous: %s  %d rounds  \"%s\"  (type /resume to continue)\n",
				t.Format("01-02 15:04"), s.Rounds, summary)
			return
		}
	}
}

// anyToMessages converts generic JSON maps back to []Message.
func anyToMessages(raw []any) ([]Message, error) {
	// Re-marshal and unmarshal through a typed struct
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var rawMsgs []struct {
		Role    string `json:"role"`
		Content []struct {
			Type      string         `json:"type"`
			Text      string         `json:"text,omitempty"`
			ID        string         `json:"id,omitempty"`
			Name      string         `json:"name,omitempty"`
			Input     map[string]any `json:"input,omitempty"`
			ToolUseID string         `json:"tool_use_id,omitempty"`
			Content   string         `json:"content,omitempty"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &rawMsgs); err != nil {
		return nil, err
	}

	var messages []Message
	for _, rm := range rawMsgs {
		msg := Message{Role: rm.Role}
		for _, b := range rm.Content {
			switch b.Type {
			case "text":
				msg.Content = append(msg.Content, ContentBlock{
					Type: "text",
					Text: b.Text,
				})
			case "tool_use":
				msg.Content = append(msg.Content, ContentBlock{
					Type:     "tool_use",
					ToolID:   b.ID,
					ToolName: b.Name,
					Input:    b.Input,
				})
			case "tool_result":
				msg.Content = append(msg.Content, ContentBlock{
					Type:     "tool_result",
					ToolID:   b.ToolUseID,
					ToolName: b.Name,
					Text:     b.Content,
				})
			}
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
