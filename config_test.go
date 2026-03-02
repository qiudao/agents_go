package main

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestJSONLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rows := []map[string]string{
		{"key": "A", "value": "1"},
		{"key": "B", "value": "2"},
	}
	if err := saveJSONL(path, rows); err != nil {
		t.Fatal(err)
	}

	got := loadJSONL(path)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0]["key"] != "A" || got[0]["value"] != "1" {
		t.Errorf("row 0 mismatch: %v", got[0])
	}
	if got[1]["key"] != "B" || got[1]["value"] != "2" {
		t.Errorf("row 1 mismatch: %v", got[1])
	}
}

func TestAppendJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	appendJSONL(path, map[string]string{"prefix": "go build"})
	appendJSONL(path, map[string]string{"prefix": "git"})

	got := loadJSONL(path)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0]["prefix"] != "go build" {
		t.Errorf("row 0: %v", got[0])
	}
	if got[1]["prefix"] != "git" {
		t.Errorf("row 1: %v", got[1])
	}
}

func TestLoadJSONLMissing(t *testing.T) {
	got := loadJSONL("/nonexistent/path.jsonl")
	if got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

func TestIsAPIKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"GEMINI_API_KEY", true},
		{"DEEPSEEK_API_KEY", true},
		{"PROVIDER", false},
		{"MODEL", false},
		{"SEARCH_CX", false},
		{"SEARCH_API_KEY", true},
	}
	for _, tc := range cases {
		if got := isAPIKey(tc.key); got != tc.want {
			t.Errorf("isAPIKey(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	home := setupTestDir(t)

	cfg := map[string]string{
		"PROVIDER":       "gemini",
		"MODEL":          "gemini-2.5-flash",
		"GEMINI_API_KEY": "test-key-123",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(home, ".config", "agents_go", "config.jsonl")); err != nil {
		t.Error("config.jsonl not created")
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "agents_go", "keys.jsonl")); err != nil {
		t.Error("keys.jsonl not created")
	}

	// Reload and verify
	got := loadConfig()
	if got["PROVIDER"] != "gemini" {
		t.Errorf("PROVIDER = %q, want gemini", got["PROVIDER"])
	}
	if got["MODEL"] != "gemini-2.5-flash" {
		t.Errorf("MODEL = %q, want gemini-2.5-flash", got["MODEL"])
	}
	if got["GEMINI_API_KEY"] != "test-key-123" {
		t.Errorf("GEMINI_API_KEY = %q, want test-key-123", got["GEMINI_API_KEY"])
	}
}

func TestAllowedCommands(t *testing.T) {
	setupTestDir(t)

	// Initially empty
	cmds := loadAllowedCommands()
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands, got %d", len(cmds))
	}

	addAllowedCommand("go build")
	addAllowedCommand("git")

	cmds = loadAllowedCommands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0] != "go build" {
		t.Errorf("cmds[0] = %q, want 'go build'", cmds[0])
	}
	if cmds[1] != "git" {
		t.Errorf("cmds[1] = %q, want 'git'", cmds[1])
	}
}

func TestMigrateOldConfig(t *testing.T) {
	home := setupTestDir(t)
	oldPath := filepath.Join(home, oldConfigFile)

	// Write old-style config
	content := "PROVIDER=deepseek\nMODEL=deepseek-chat\nGEMINI_API_KEY=key123\nALLOWED_COMMANDS=go build,git\n"
	os.WriteFile(oldPath, []byte(content), 0644)

	// Trigger migration via loadConfig
	cfg := loadConfig()

	// Config values loaded
	if cfg["PROVIDER"] != "deepseek" {
		t.Errorf("PROVIDER = %q", cfg["PROVIDER"])
	}
	if cfg["GEMINI_API_KEY"] != "key123" {
		t.Errorf("GEMINI_API_KEY = %q", cfg["GEMINI_API_KEY"])
	}

	// Permissions migrated
	cmds := loadAllowedCommands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0] != "go build" || cmds[1] != "git" {
		t.Errorf("commands = %v", cmds)
	}

	// Old file renamed to .bak
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old config file should have been renamed")
	}
	if _, err := os.Stat(oldPath + ".bak"); err != nil {
		t.Error("backup file should exist")
	}
}

func TestMigrateNoOldFile(t *testing.T) {
	setupTestDir(t)

	// Should not panic or error when no old file exists
	cfg := loadConfig()
	if len(cfg) != 0 {
		t.Errorf("expected empty config, got %v", cfg)
	}
}
