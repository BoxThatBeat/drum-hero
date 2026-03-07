package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(path, []byte("test audio content"), 0o644)

	hash, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile() error: %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(hash))
	}

	// Hash should be deterministic
	hash2, _ := HashFile(path)
	if hash != hash2 {
		t.Error("hash should be deterministic")
	}
}

func TestDir(t *testing.T) {
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	dir := Dir(hash)
	if !strings.HasSuffix(dir, "abcdef1234567890") {
		t.Errorf("expected dir to end with first 16 chars of hash, got %s", dir)
	}
}

func TestPaths(t *testing.T) {
	hash := "abcdef1234567890"
	if !strings.HasSuffix(DrumsPath(hash), "drums.wav") {
		t.Error("DrumsPath should end with drums.wav")
	}
	if !strings.HasSuffix(NoDrumsPath(hash), "no_drums.wav") {
		t.Error("NoDrumsPath should end with no_drums.wav")
	}
	if !strings.HasSuffix(DrumMapPath(hash), "drummap.json") {
		t.Error("DrumMapPath should end with drummap.json")
	}
	if !strings.HasSuffix(MetaPath(hash), "meta.json") {
		t.Error("MetaPath should end with meta.json")
	}
}

func TestExistsAndWriteMeta(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	hash := "abc123testcachehash"

	if Exists(hash) {
		t.Error("cache should not exist initially")
	}

	meta := Meta{
		OriginalFile: "/home/user/song.mp3",
		SHA256:       hash,
		Model:        "htdemucs_ft",
	}

	if err := WriteMeta(hash, meta); err != nil {
		t.Fatalf("WriteMeta() error: %v", err)
	}

	if !Exists(hash) {
		t.Error("cache should exist after WriteMeta")
	}

	readMeta, err := ReadMeta(hash)
	if err != nil {
		t.Fatalf("ReadMeta() error: %v", err)
	}

	if readMeta.OriginalFile != meta.OriginalFile {
		t.Errorf("expected %s, got %s", meta.OriginalFile, readMeta.OriginalFile)
	}
	if readMeta.Model != meta.Model {
		t.Errorf("expected %s, got %s", meta.Model, readMeta.Model)
	}
}

func TestHasSeparation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	hash := "testseparation12"

	if HasSeparation(hash) {
		t.Error("should not have separation initially")
	}

	EnsureDir(hash)
	os.WriteFile(DrumsPath(hash), []byte("drums"), 0o644)
	os.WriteFile(NoDrumsPath(hash), []byte("no_drums"), 0o644)

	if !HasSeparation(hash) {
		t.Error("should have separation after creating files")
	}
}

func TestHasDrumMap(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	hash := "testdrummap12345"

	if HasDrumMap(hash) {
		t.Error("should not have drum map initially")
	}

	EnsureDir(hash)
	os.WriteFile(DrumMapPath(hash), []byte("[]"), 0o644)

	if !HasDrumMap(hash) {
		t.Error("should have drum map after creating file")
	}
}
