package main

import (
	"strings"
	"testing"
)

func newTodo() *TodoManager {
	return &TodoManager{}
}

// --- Update: basic ---

func TestTodoUpdateBasic(t *testing.T) {
	tm := newTodo()
	result, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "do thing", "status": "pending"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "[ ] #1: do thing") {
		t.Errorf("unexpected render: %s", result)
	}
	if !strings.Contains(result, "(0/1 completed)") {
		t.Errorf("unexpected count: %s", result)
	}
}

func TestTodoUpdateMultipleStatuses(t *testing.T) {
	tm := newTodo()
	result, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "done task", "status": "completed"},
		map[string]any{"id": "2", "text": "current task", "status": "in_progress"},
		map[string]any{"id": "3", "text": "next task", "status": "pending"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "[x] #1") {
		t.Errorf("missing completed marker: %s", result)
	}
	if !strings.Contains(result, "[>] #2") {
		t.Errorf("missing in_progress marker: %s", result)
	}
	if !strings.Contains(result, "[ ] #3") {
		t.Errorf("missing pending marker: %s", result)
	}
	if !strings.Contains(result, "(1/3 completed)") {
		t.Errorf("unexpected count: %s", result)
	}
}

func TestTodoUpdateReplacesAll(t *testing.T) {
	tm := newTodo()
	tm.Update([]any{
		map[string]any{"id": "1", "text": "old", "status": "pending"},
		map[string]any{"id": "2", "text": "old2", "status": "pending"},
	})
	result, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "new", "status": "completed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tm.items) != 1 {
		t.Errorf("expected 1 item after replace, got %d", len(tm.items))
	}
	if !strings.Contains(result, "new") {
		t.Errorf("expected new text: %s", result)
	}
}

// --- Update: validation ---

func TestTodoUpdateEmptyText(t *testing.T) {
	tm := newTodo()
	_, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "", "status": "pending"},
	})
	if err == nil {
		t.Error("expected error for empty text")
	}
	if !strings.Contains(err.Error(), "text required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTodoUpdateInvalidStatus(t *testing.T) {
	tm := newTodo()
	_, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "task", "status": "done"},
	})
	if err == nil {
		t.Error("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTodoUpdateMultipleInProgress(t *testing.T) {
	tm := newTodo()
	_, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "task A", "status": "in_progress"},
		map[string]any{"id": "2", "text": "task B", "status": "in_progress"},
	})
	if err == nil {
		t.Error("expected error for multiple in_progress")
	}
	if !strings.Contains(err.Error(), "only one") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTodoUpdateMaxItems(t *testing.T) {
	tm := newTodo()
	items := make([]any, 21)
	for i := range items {
		items[i] = map[string]any{"id": "x", "text": "task", "status": "pending"}
	}
	_, err := tm.Update(items)
	if err == nil {
		t.Error("expected error for >20 items")
	}
	if !strings.Contains(err.Error(), "max 20") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Update: defaults ---

func TestTodoUpdateDefaultID(t *testing.T) {
	tm := newTodo()
	_, err := tm.Update([]any{
		map[string]any{"text": "no id", "status": "pending"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tm.items[0].ID != "1" {
		t.Errorf("expected default id '1', got %q", tm.items[0].ID)
	}
}

func TestTodoUpdateDefaultStatus(t *testing.T) {
	tm := newTodo()
	_, err := tm.Update([]any{
		map[string]any{"id": "1", "text": "no status"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tm.items[0].Status != "pending" {
		t.Errorf("expected default status 'pending', got %q", tm.items[0].Status)
	}
}

// --- Render ---

func TestRenderEmpty(t *testing.T) {
	tm := newTodo()
	if tm.Render() != "No todos." {
		t.Errorf("unexpected: %s", tm.Render())
	}
}
