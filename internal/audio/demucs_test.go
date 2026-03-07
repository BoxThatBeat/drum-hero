package audio

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	content := []byte("hello world")
	os.WriteFile(src, content, 0o644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestScanLines(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"hello\nworld\n", []string{"hello", "world"}},
		{"hello\r\nworld\r\n", []string{"hello", "world"}},
		{"progress\rprogress2\rprogress3\n", []string{"progress", "progress2", "progress3"}},
	}

	for _, tt := range tests {
		reader := strings.NewReader(tt.input)
		scanner := bufio.NewScanner(reader)
		scanner.Split(scanLines)

		var tokens []string
		for scanner.Scan() {
			text := scanner.Text()
			if text != "" {
				tokens = append(tokens, text)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("scanner error for %q: %v", tt.input, err)
		}

		if len(tokens) != len(tt.expected) {
			t.Errorf("input %q: expected %d tokens, got %d: %v", tt.input, len(tt.expected), len(tokens), tokens)
			continue
		}
		for i, tok := range tokens {
			if tok != tt.expected[i] {
				t.Errorf("input %q: token %d: expected %q, got %q", tt.input, i, tt.expected[i], tok)
			}
		}
	}
}

func TestCheckDemucs(t *testing.T) {
	// This test just verifies the function doesn't panic
	// It may pass or fail depending on whether demucs is installed
	_ = CheckDemucs()
}
