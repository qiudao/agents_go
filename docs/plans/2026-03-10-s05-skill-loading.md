# s05 Skill Loading Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add two-layer skill injection to agents_go — metadata in system prompt, full body via `load_skill` tool.

**Architecture:** SkillLoader scans `~/.config/agents_go/skills/*/SKILL.md` at startup, parses YAML frontmatter for metadata (Layer 1 → system prompt), and serves full body on demand via `load_skill` tool_result (Layer 2). One new file `skill.go`, minor edits to `s01_agent_loop.go`.

**Tech Stack:** Go stdlib only (no YAML library — hand-parse `---` frontmatter like the Python version).

---

### Task 1: SkillLoader with frontmatter parsing

**Files:**
- Create: `skill.go`
- Test: `skill_test.go`

**Step 1: Write the failing test**

```go
// skill_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillLoaderEmpty(t *testing.T) {
	dir := t.TempDir()
	sl := NewSkillLoader(dir)
	if len(sl.skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(sl.skills))
	}
	if sl.GetDescriptions() != "(no skills available)" {
		t.Fatalf("unexpected descriptions: %s", sl.GetDescriptions())
	}
}

func TestSkillLoaderSingleSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "pdf")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: pdf\ndescription: Process PDF files\n---\n\n# PDF Skill\n\nDo PDF things."), 0o644)

	sl := NewSkillLoader(dir)
	if len(sl.skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(sl.skills))
	}
	desc := sl.GetDescriptions()
	if desc != "  - pdf: Process PDF files" {
		t.Fatalf("unexpected descriptions: %q", desc)
	}
	content := sl.GetContent("pdf")
	if content != "<skill name=\"pdf\">\n# PDF Skill\n\nDo PDF things.\n</skill>" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestSkillLoaderUnknown(t *testing.T) {
	dir := t.TempDir()
	sl := NewSkillLoader(dir)
	content := sl.GetContent("nope")
	if content != "Error: Unknown skill 'nope'. No skills available." {
		t.Fatalf("unexpected: %q", content)
	}
}

func TestSkillLoaderNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "raw")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Just a body, no frontmatter."), 0o644)

	sl := NewSkillLoader(dir)
	if len(sl.skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(sl.skills))
	}
	// Falls back to directory name, no description
	desc := sl.GetDescriptions()
	if desc != "  - raw: No description" {
		t.Fatalf("unexpected descriptions: %q", desc)
	}
}

func TestSkillLoaderMultiple(t *testing.T) {
	dir := t.TempDir()
	for _, s := range []struct{ name, desc string }{
		{"alpha", "First skill"},
		{"beta", "Second skill"},
	} {
		d := filepath.Join(dir, s.name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("---\nname: "+s.name+"\ndescription: "+s.desc+"\n---\n\nBody of "+s.name), 0o644)
	}
	sl := NewSkillLoader(dir)
	if len(sl.skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(sl.skills))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/k/work/agents_go && go test -run TestSkillLoader -v`
Expected: FAIL — `NewSkillLoader` not defined

**Step 3: Write minimal implementation**

```go
// skill.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Body        string
	Path        string
}

type SkillLoader struct {
	skillsDir string
	skills    map[string]Skill
	order     []string // sorted names for stable output
}

func NewSkillLoader(skillsDir string) *SkillLoader {
	sl := &SkillLoader{
		skillsDir: skillsDir,
		skills:    make(map[string]Skill),
	}
	sl.loadAll()
	return sl
}

func (sl *SkillLoader) loadAll() {
	entries, err := os.ReadDir(sl.skillsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(sl.skillsDir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name, desc, body := parseFrontmatter(string(data))
		if name == "" {
			name = e.Name()
		}
		sl.skills[name] = Skill{
			Name:        name,
			Description: desc,
			Body:        body,
			Path:        path,
		}
	}
	sl.order = make([]string, 0, len(sl.skills))
	for name := range sl.skills {
		sl.order = append(sl.order, name)
	}
	sort.Strings(sl.order)
}

// parseFrontmatter extracts name, description, and body from YAML frontmatter.
func parseFrontmatter(text string) (name, description, body string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", "", strings.TrimSpace(text)
	}
	rest := text[4:] // skip opening "---\n"
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		return "", "", strings.TrimSpace(text)
	}
	header := rest[:end]
	body = strings.TrimSpace(rest[end+4:])

	for _, line := range strings.Split(header, "\n") {
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch key {
		case "name":
			name = val
		case "description":
			description = val
		}
	}
	return
}

func (sl *SkillLoader) GetDescriptions() string {
	if len(sl.skills) == 0 {
		return "(no skills available)"
	}
	var lines []string
	for _, name := range sl.order {
		s := sl.skills[name]
		desc := s.Description
		if desc == "" {
			desc = "No description"
		}
		lines = append(lines, fmt.Sprintf("  - %s: %s", name, desc))
	}
	return strings.Join(lines, "\n")
}

func (sl *SkillLoader) GetContent(name string) string {
	s, ok := sl.skills[name]
	if !ok {
		if len(sl.skills) == 0 {
			return fmt.Sprintf("Error: Unknown skill '%s'. No skills available.", name)
		}
		return fmt.Sprintf("Error: Unknown skill '%s'. Available: %s", name, strings.Join(sl.order, ", "))
	}
	return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", name, s.Body)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/k/work/agents_go && go test -run TestSkillLoader -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/k/work/agents_go
git add skill.go skill_test.go
git commit -m "feat: add SkillLoader with YAML frontmatter parsing (s05)"
```

---

### Task 2: Integrate into agent loop

**Files:**
- Modify: `s01_agent_loop.go:62-126` (executeTool + tools)
- Modify: `s01_agent_loop.go:433-458` (main — init loader, update system prompt)

**Step 1: Add `load_skill` tool definition**

In `s01_agent_loop.go`, add `load_skill` to `tools` (after the `todo` tool, same pattern as `task`/`todo` appended to `childTools`):

```go
// After the existing tools = append(childTools, ...) block, append one more:
// Find the closing paren of: var tools = append(childTools, Tool{...}, Tool{...})
// Add this Tool to that list:
Tool{
    Name:        "load_skill",
    Description: "Load specialized knowledge by name. Use this before tackling unfamiliar topics.",
    Properties: map[string]any{
        "name": map[string]any{
            "type":        "string",
            "description": "Skill name to load",
        },
    },
},
```

**Step 2: Add package-level SkillLoader and init in main()**

At package level (near `var todo = &TodoManager{}`):

```go
var skillLoader *SkillLoader
```

In `main()`, after `cwd, _ := os.Getwd()` and before building `system`:

```go
skillLoader = NewSkillLoader(filepath.Join(configDir(), "skills"))
```

Update the `system` string to append skill descriptions:

```go
system := fmt.Sprintf("You are a coding agent at %s. Use the todo tool to plan multi-step tasks. Mark in_progress before starting, completed when done. Prefer tools over prose.", cwd)
if descs := skillLoader.GetDescriptions(); descs != "(no skills available)" {
    system += "\n\nSkills available (use load_skill to access):\n" + descs
}
```

**Step 3: Add dispatch case in executeTool()**

```go
case "load_skill":
    name, _ := b.Input["name"].(string)
    fmt.Printf("\033[33m📚 load_skill(%s)\033[0m\n", name)
    return skillLoader.GetContent(name)
```

**Step 4: Add `"path/filepath"` to imports if not already present**

Check imports at top of `s01_agent_loop.go`, add `"path/filepath"` if missing.

**Step 5: Build and verify**

Run: `cd /Users/k/work/agents_go && go build ./...`
Expected: No errors

**Step 6: Run all tests**

Run: `cd /Users/k/work/agents_go && go test ./... -v`
Expected: All PASS (existing + new skill tests)

**Step 7: Commit**

```bash
cd /Users/k/work/agents_go
git add s01_agent_loop.go
git commit -m "feat: integrate load_skill tool into agent loop (s05)"
```

---

### Task 3: Add example skill and smoke test

**Files:**
- Create: `~/.config/agents_go/skills/pdf/SKILL.md` (example skill)

**Step 1: Create example skill directory**

```bash
mkdir -p ~/.config/agents_go/skills/pdf
```

**Step 2: Write example SKILL.md**

```bash
cat > ~/.config/agents_go/skills/pdf/SKILL.md << 'EOF'
---
name: pdf
description: Process PDF files - extract text, create PDFs, merge documents.
---

# PDF Processing Skill

## Reading PDFs
```bash
pdftotext input.pdf -          # stdout
pdftotext input.pdf output.txt # to file
```

## Creating PDFs
```bash
pandoc input.md -o output.pdf
```

## Key tools
- pdftotext (poppler-utils): text extraction
- pandoc: markdown to PDF
- pymupdf: programmatic read/write/merge
EOF
```

**Step 3: Manual smoke test**

Run: `cd /Users/k/work/agents_go && go run . <<< "What skills are available?"`
Expected: Agent mentions "pdf" skill in its response.

Run: `cd /Users/k/work/agents_go && go run . <<< "Load the pdf skill"`
Expected: Agent calls `load_skill("pdf")`, gets full body back.

**Step 4: Commit**

```bash
cd /Users/k/work/agents_go
git add -A  # only if you want the example skill tracked; skip if ~/.config is outside repo
git commit -m "docs: add example pdf skill for smoke testing (s05)"
```
