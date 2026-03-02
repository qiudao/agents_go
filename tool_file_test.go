package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- safePath ---

func TestSafePathAbsolute(t *testing.T) {
	got, err := safePath("/tmp/foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/foo.txt" {
		t.Errorf("got %q, want /tmp/foo.txt", got)
	}
}

func TestSafePathRelative(t *testing.T) {
	got, err := safePath("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(workdir, "foo.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSafePathDotDot(t *testing.T) {
	got, err := safePath("/tmp/a/../b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/b.txt" {
		t.Errorf("got %q, want /tmp/b.txt", got)
	}
}

// --- isBinary ---

func TestIsBinaryText(t *testing.T) {
	f := filepath.Join(t.TempDir(), "text.txt")
	os.WriteFile(f, []byte("hello world\n"), 0644)
	bin, err := isBinary(f)
	if err != nil {
		t.Fatal(err)
	}
	if bin {
		t.Error("text file detected as binary")
	}
}

func TestIsBinaryWithNull(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bin.dat")
	os.WriteFile(f, []byte("hello\x00world"), 0644)
	bin, err := isBinary(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bin {
		t.Error("binary file not detected")
	}
}

func TestIsBinaryEmpty(t *testing.T) {
	f := filepath.Join(t.TempDir(), "empty")
	os.WriteFile(f, []byte{}, 0644)
	bin, err := isBinary(f)
	if err != nil {
		t.Fatal(err)
	}
	if bin {
		t.Error("empty file detected as binary")
	}
}

func TestIsBinaryMissing(t *testing.T) {
	_, err := isBinary("/nonexistent/file")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- runRead ---

func TestRunReadBasic(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(f, []byte("line1\nline2\nline3\n"), 0644)
	out := runRead(f, 0, 0)
	if !strings.Contains(out, "1: line1") {
		t.Errorf("missing line1: %s", out)
	}
	if !strings.Contains(out, "3: line3") {
		t.Errorf("missing line3: %s", out)
	}
}

func TestRunReadWithLimit(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(f, []byte("a\nb\nc\nd\ne\n"), 0644)
	out := runRead(f, 2, 0)
	if !strings.Contains(out, "1: a") {
		t.Errorf("missing line 1: %s", out)
	}
	if !strings.Contains(out, "2: b") {
		t.Errorf("missing line 2: %s", out)
	}
	if !strings.Contains(out, "more lines") {
		t.Errorf("missing truncation notice: %s", out)
	}
	if strings.Contains(out, "3: c") {
		t.Errorf("should not contain line 3: %s", out)
	}
}

func TestRunReadWithOffset(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(f, []byte("a\nb\nc\nd\n"), 0644)
	out := runRead(f, 0, 2)
	// Should start at line 3 (offset=2 skips first 2)
	if !strings.Contains(out, "3: c") {
		t.Errorf("expected line 3: %s", out)
	}
	if strings.Contains(out, "1: a") {
		t.Errorf("should not contain line 1: %s", out)
	}
}

func TestRunReadWithOffsetAndLimit(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(f, []byte("a\nb\nc\nd\ne\n"), 0644)
	out := runRead(f, 1, 2)
	if !strings.Contains(out, "3: c") {
		t.Errorf("expected line 3: %s", out)
	}
	if !strings.Contains(out, "more lines") {
		t.Errorf("missing truncation notice: %s", out)
	}
}

func TestRunReadOffsetExceedsLength(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(f, []byte("a\nb\n"), 0644)
	out := runRead(f, 0, 100)
	if !strings.Contains(out, "Error") {
		t.Errorf("expected error for large offset: %s", out)
	}
}

func TestRunReadMissingFile(t *testing.T) {
	out := runRead("/nonexistent/path.txt", 0, 0)
	if !strings.Contains(out, "Error") {
		t.Errorf("expected error: %s", out)
	}
}

func TestRunReadBinaryRejected(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bin.dat")
	os.WriteFile(f, []byte("data\x00\x01\x02"), 0644)
	out := runRead(f, 0, 0)
	if !strings.Contains(out, "binary") {
		t.Errorf("expected binary rejection: %s", out)
	}
}

// --- runWrite ---

func TestRunWriteBasic(t *testing.T) {
	f := filepath.Join(t.TempDir(), "out.txt")
	out := runWrite(f, "hello")
	if !strings.Contains(out, "Wrote 5 bytes") {
		t.Errorf("unexpected output: %s", out)
	}
	data, _ := os.ReadFile(f)
	if string(data) != "hello" {
		t.Errorf("file content = %q", data)
	}
}

func TestRunWriteCreatesDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "sub", "dir", "out.txt")
	out := runWrite(f, "nested")
	if strings.Contains(out, "Error") {
		t.Errorf("unexpected error: %s", out)
	}
	data, _ := os.ReadFile(f)
	if string(data) != "nested" {
		t.Errorf("file content = %q", data)
	}
}

func TestRunWriteTooLarge(t *testing.T) {
	f := filepath.Join(t.TempDir(), "big.txt")
	big := strings.Repeat("x", maxWriteSize+1)
	out := runWrite(f, big)
	if !strings.Contains(out, "too large") {
		t.Errorf("expected size error: %s", out)
	}
}

// --- runEdit ---

func TestRunEditBasic(t *testing.T) {
	f := filepath.Join(t.TempDir(), "edit.txt")
	os.WriteFile(f, []byte("hello world"), 0644)
	out := runEdit(f, "world", "go")
	if !strings.Contains(out, "Edited") {
		t.Errorf("unexpected output: %s", out)
	}
	data, _ := os.ReadFile(f)
	if string(data) != "hello go" {
		t.Errorf("file content = %q", data)
	}
}

func TestRunEditNotFound(t *testing.T) {
	f := filepath.Join(t.TempDir(), "edit.txt")
	os.WriteFile(f, []byte("hello world"), 0644)
	out := runEdit(f, "missing", "x")
	if !strings.Contains(out, "not found") {
		t.Errorf("expected not found error: %s", out)
	}
}

func TestRunEditMultipleMatches(t *testing.T) {
	f := filepath.Join(t.TempDir(), "edit.txt")
	os.WriteFile(f, []byte("aaa bbb aaa"), 0644)
	out := runEdit(f, "aaa", "ccc")
	if !strings.Contains(out, "matches 2 locations") {
		t.Errorf("expected uniqueness error: %s", out)
	}
	// File should be unchanged
	data, _ := os.ReadFile(f)
	if string(data) != "aaa bbb aaa" {
		t.Errorf("file was modified despite error: %q", data)
	}
}

func TestRunEditBinaryRejected(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bin.dat")
	os.WriteFile(f, []byte("abc\x00def"), 0644)
	out := runEdit(f, "abc", "xyz")
	if !strings.Contains(out, "binary") {
		t.Errorf("expected binary rejection: %s", out)
	}
}

func TestRunEditMissingFile(t *testing.T) {
	out := runEdit("/nonexistent/file.txt", "a", "b")
	if !strings.Contains(out, "Error") {
		t.Errorf("expected error: %s", out)
	}
}

// --- commandPrefix / isCommandAllowed ---

func TestCommandPrefix(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"go build ./...", "go build"},
		{"git status", "git status"},
		{"ls -la", "ls"},
		{"npm install foo", "npm install"},
		{"echo hello", "echo"},
		{"", ""},
	}
	for _, tc := range cases {
		got := commandPrefix(tc.cmd)
		if got != tc.want {
			t.Errorf("commandPrefix(%q) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

func TestIsCommandAllowed(t *testing.T) {
	allowed := []string{"go build", "git", "ls"}

	cases := []struct {
		cmd  string
		want bool
	}{
		{"go build ./...", true},
		{"git status", true},
		{"git push", true},
		{"ls -la", true},
		{"rm -rf /", false},
		{"sudo reboot", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isCommandAllowed(tc.cmd, allowed)
		if got != tc.want {
			t.Errorf("isCommandAllowed(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}
