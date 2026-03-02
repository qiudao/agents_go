package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var workdir string

func init() {
	workdir, _ = os.Getwd()
}

// safePath resolves p to an absolute path. Accepts both relative and absolute paths.
func safePath(p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(workdir, p)
	}
	return filepath.Clean(p), nil
}

const (
	maxReadSize  = 10 * 1024 * 1024 // 10 MB
	maxWriteSize = 1 * 1024 * 1024  // 1 MB
	binaryProbe  = 8 * 1024         // 8 KB
)

// isBinary checks the first 8KB of a file for null bytes (Git-style detection).
func isBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, binaryProbe)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true, nil
		}
	}
	return false, nil
}

func runRead(path string, limit, offset int) string {
	fp, err := safePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Size check
	info, err := os.Stat(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if info.Size() > maxReadSize {
		return fmt.Sprintf("Error: file too large (%d bytes, max %d). Use bash to read it.", info.Size(), maxReadSize)
	}

	// Binary check
	bin, err := isBinary(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if bin {
		return fmt.Sprintf("Error: %s is a binary file. Use bash to inspect it.", path)
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	// Apply offset
	if offset > 0 {
		if offset >= len(lines) {
			return fmt.Sprintf("Error: offset %d exceeds file length (%d lines)", offset, len(lines))
		}
		lines = lines[offset:]
	}

	if limit > 0 && limit < len(lines) {
		remaining := len(lines) - limit
		lines = append(lines[:limit], fmt.Sprintf("... (%d more lines)", remaining))
	}

	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%d: %s\n", offset+i+1, line)
	}
	out := sb.String()
	if len(out) > 50000 {
		out = out[:50000]
	}
	return out
}

func runWrite(path, content string) string {
	if len(content) > maxWriteSize {
		return fmt.Sprintf("Error: content too large (%d bytes, max %d). Use bash to write large files.", len(content), maxWriteSize)
	}
	fp, err := safePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path)
}

func runEdit(path, oldText, newText string) string {
	fp, err := safePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Binary check
	bin, err := isBinary(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if bin {
		return fmt.Sprintf("Error: %s is a binary file. Use bash to modify it.", path)
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	content := string(data)

	// Uniqueness check: old_text must appear exactly once
	count := strings.Count(content, oldText)
	if count == 0 {
		return fmt.Sprintf("Error: text not found in %s", path)
	}
	if count > 1 {
		return fmt.Sprintf("Error: old_text matches %d locations in %s. Include more surrounding context to make it unique.", count, path)
	}

	content = strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Edited %s", path)
}
