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
