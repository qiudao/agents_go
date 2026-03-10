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
