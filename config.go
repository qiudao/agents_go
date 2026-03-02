package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const oldConfigFile = ".agents_go.config"

type ModelOption struct {
	Provider string // "gemini" or "anthropic"
	Model    string
	Label    string // display name
}

var availableModels = []ModelOption{
	{"gemini", "gemini-2.5-flash", "Gemini 2.5 Flash"},
	{"gemini", "gemini-2.5-pro", "Gemini 2.5 Pro"},
	{"deepseek", "deepseek-chat", "DeepSeek V3"},
	{"deepseek", "deepseek-reasoner", "DeepSeek R1"},
	{"anthropic", "claude-sonnet-4-6", "Claude Sonnet 4.6"},
	{"anthropic", "claude-opus-4-6", "Claude Opus 4.6"},
	{"qwen", "qwen-turbo", "Qwen Turbo"},
	{"qwen", "qwen-plus", "Qwen Plus"},
	{"qwen", "qwen-max", "Qwen Max"},
}

// --- path helpers ---

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agents_go")
}

func configFilePath() string  { return filepath.Join(configDir(), "config.jsonl") }
func keysFilePath() string    { return filepath.Join(configDir(), "keys.jsonl") }
func permissionsFilePath() string { return filepath.Join(configDir(), "permissions.jsonl") }

// configPath returns the config directory path (used in user-facing messages).
func configPath() string { return configDir() }

// --- generic JSONL read/write ---

func loadJSONL(path string) []map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows []map[string]string
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var m map[string]string
		if json.Unmarshal([]byte(line), &m) == nil {
			rows = append(rows, m)
		}
	}
	return rows
}

func saveJSONL(path string, rows []map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var lines []string
	for _, row := range rows {
		b, err := json.Marshal(row)
		if err != nil {
			return err
		}
		lines = append(lines, string(b))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func appendJSONL(path string, obj map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, string(b))
	return err
}

// --- key routing ---

func isAPIKey(key string) bool {
	return strings.HasSuffix(key, "_API_KEY")
}

// --- migration from old flat file ---

func migrateOldConfig() {
	home, _ := os.UserHomeDir()
	oldPath := filepath.Join(home, oldConfigFile)

	if _, err := os.Stat(oldPath); err != nil {
		return // no old file
	}

	f, err := os.Open(oldPath)
	if err != nil {
		return
	}
	defer f.Close()

	var configRows, keyRows []map[string]string
	var permPrefixes []string

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		if k == "ALLOWED_COMMANDS" {
			for _, cmd := range strings.Split(v, ",") {
				cmd = strings.TrimSpace(cmd)
				if cmd != "" {
					permPrefixes = append(permPrefixes, cmd)
				}
			}
			continue
		}

		row := map[string]string{"key": k, "value": v}
		if isAPIKey(k) {
			keyRows = append(keyRows, row)
		} else {
			configRows = append(configRows, row)
		}
	}

	os.MkdirAll(configDir(), 0755)

	if len(configRows) > 0 {
		saveJSONL(configFilePath(), configRows)
	}
	if len(keyRows) > 0 {
		saveJSONL(keysFilePath(), keyRows)
	}
	if len(permPrefixes) > 0 {
		var permRows []map[string]string
		for _, p := range permPrefixes {
			permRows = append(permRows, map[string]string{"prefix": p})
		}
		saveJSONL(permissionsFilePath(), permRows)
	}

	os.Rename(oldPath, oldPath+".bak")
}

// --- public config functions (signatures unchanged) ---

// loadConfig reads config.jsonl + keys.jsonl into a unified map.
// On first call, migrates from old flat file if present.
func loadConfig() map[string]string {
	migrateOldConfig()

	cfg := make(map[string]string)
	for _, row := range loadJSONL(configFilePath()) {
		if k, ok := row["key"]; ok {
			cfg[k] = row["value"]
		}
	}
	for _, row := range loadJSONL(keysFilePath()) {
		if k, ok := row["key"]; ok {
			cfg[k] = row["value"]
		}
	}
	return cfg
}

// saveConfig splits cfg by key type and writes to config.jsonl / keys.jsonl.
func saveConfig(cfg map[string]string) error {
	var configRows, keyRows []map[string]string
	for k, v := range cfg {
		row := map[string]string{"key": k, "value": v}
		if isAPIKey(k) {
			keyRows = append(keyRows, row)
		} else {
			configRows = append(configRows, row)
		}
	}
	if err := saveJSONL(configFilePath(), configRows); err != nil {
		return err
	}
	return saveJSONL(keysFilePath(), keyRows)
}

// loadAllowedCommands reads command prefixes from permissions.jsonl.
func loadAllowedCommands() []string {
	migrateOldConfig()

	var result []string
	for _, row := range loadJSONL(permissionsFilePath()) {
		if p, ok := row["prefix"]; ok && p != "" {
			result = append(result, p)
		}
	}
	return result
}

// addAllowedCommand appends a command prefix to permissions.jsonl.
func addAllowedCommand(prefix string) {
	appendJSONL(permissionsFilePath(), map[string]string{"prefix": prefix})
}

// isCommandAllowed checks if a command matches any allowed prefix.
func isCommandAllowed(command string, allowed []string) bool {
	cmd := strings.TrimSpace(command)
	for _, prefix := range allowed {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

// commandPrefix extracts the first word (or first two words for common patterns) as prefix.
func commandPrefix(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return command
	}
	// For common tools, use first two words as prefix (e.g. "go build", "git status")
	if len(fields) >= 2 {
		switch fields[0] {
		case "go", "git", "npm", "npx", "cargo", "make", "docker", "kubectl":
			return fields[0] + " " + fields[1]
		}
	}
	return fields[0]
}

// selectModel shows an interactive menu, returns the chosen option.
// Returns nil if user cancels.
func selectModel(rl interface{ Readline() (string, error) }) *ModelOption {
	fmt.Println("\nAvailable models:")
	for i, m := range availableModels {
		fmt.Printf("  %d) %-10s / %s\n", i+1, m.Provider, m.Label)
	}
	fmt.Print("\nSelect [1-", len(availableModels), "]: ")

	line, err := rl.Readline()
	if err != nil {
		return nil
	}
	line = strings.TrimSpace(line)
	var idx int
	if _, err := fmt.Sscanf(line, "%d", &idx); err != nil || idx < 1 || idx > len(availableModels) {
		fmt.Println("Invalid selection")
		return nil
	}
	return &availableModels[idx-1]
}
