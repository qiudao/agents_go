package main

import (
	"fmt"
	"strings"
)

// TodoItem represents a single task in the todo list.
type TodoItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"` // "pending", "in_progress", "completed"
}

// TodoManager holds the agent's self-managed task list.
// Only one item can be in_progress at a time.
type TodoManager struct {
	items []TodoItem
}

var todo = &TodoManager{}

// Update replaces the entire todo list with validated items.
func (tm *TodoManager) Update(raw []any) (string, error) {
	if len(raw) > 20 {
		return "", fmt.Errorf("max 20 todos allowed")
	}

	var validated []TodoItem
	inProgressCount := 0

	for i, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			return "", fmt.Errorf("item %d: expected object", i)
		}

		id, _ := m["id"].(string)
		if id == "" {
			id = fmt.Sprintf("%d", i+1)
		}
		text, _ := m["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			return "", fmt.Errorf("item #%s: text required", id)
		}
		status, _ := m["status"].(string)
		status = strings.ToLower(strings.TrimSpace(status))
		if status == "" {
			status = "pending"
		}

		switch status {
		case "pending", "in_progress", "completed":
		default:
			return "", fmt.Errorf("item #%s: invalid status %q", id, status)
		}

		if status == "in_progress" {
			inProgressCount++
		}
		validated = append(validated, TodoItem{ID: id, Text: text, Status: status})
	}

	if inProgressCount > 1 {
		return "", fmt.Errorf("only one task can be in_progress at a time")
	}

	tm.items = validated
	return tm.Render(), nil
}

// Render returns a human-readable view of the todo list.
func (tm *TodoManager) Render() string {
	if len(tm.items) == 0 {
		return "No todos."
	}

	markers := map[string]string{
		"pending":     "[ ]",
		"in_progress": "[>]",
		"completed":   "[x]",
	}

	var lines []string
	done := 0
	for _, item := range tm.items {
		lines = append(lines, fmt.Sprintf("%s #%s: %s", markers[item.Status], item.ID, item.Text))
		if item.Status == "completed" {
			done++
		}
	}
	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", done, len(tm.items)))
	return strings.Join(lines, "\n")
}
