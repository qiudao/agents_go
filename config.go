package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configFile = ".agents_go.config"

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

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configFile)
}

// loadConfig reads ~/.agents_go.config into a map.
func loadConfig() map[string]string {
	cfg := make(map[string]string)
	f, err := os.Open(configPath())
	if err != nil {
		return cfg
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			cfg[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return cfg
}

// saveConfig writes key=value pairs to ~/.agents_go.config.
func saveConfig(cfg map[string]string) error {
	var lines []string
	for k, v := range cfg {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	return os.WriteFile(configPath(), []byte(strings.Join(lines, "\n")+"\n"), 0644)
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
