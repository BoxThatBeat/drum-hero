package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Keys.Kick != "j" {
		t.Errorf("expected kick=j, got %s", cfg.Keys.Kick)
	}
	if cfg.Keys.Snare != "f" {
		t.Errorf("expected snare=f, got %s", cfg.Keys.Snare)
	}
	if cfg.Difficulty.Preset != Medium {
		t.Errorf("expected medium difficulty, got %s", cfg.Difficulty.Preset)
	}
}

func TestKeyToDrum(t *testing.T) {
	cfg := DefaultConfig()
	m := cfg.KeyToDrum()

	if m["j"] != Kick {
		t.Errorf("expected j->kick, got %s", m["j"])
	}
	if m["f"] != Snare {
		t.Errorf("expected f->snare, got %s", m["f"])
	}
	if m[";"] != Cymbal {
		t.Errorf("expected ;->cymbal, got %s", m[";"])
	}
}

func TestDrumToKey(t *testing.T) {
	cfg := DefaultConfig()
	m := cfg.DrumToKey()

	if m[Kick] != "j" {
		t.Errorf("expected kick->j, got %s", m[Kick])
	}
}

func TestThresholdMs(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ThresholdMs() != 100 {
		t.Errorf("expected 100ms for medium, got %d", cfg.ThresholdMs())
	}

	cfg.Difficulty.Preset = Easy
	if cfg.ThresholdMs() != 150 {
		t.Errorf("expected 150ms for easy, got %d", cfg.ThresholdMs())
	}

	cfg.Difficulty.Preset = Hard
	if cfg.ThresholdMs() != 60 {
		t.Errorf("expected 60ms for hard, got %d", cfg.ThresholdMs())
	}

	cfg.Difficulty.Preset = Expert
	if cfg.ThresholdMs() != 30 {
		t.Errorf("expected 30ms for expert, got %d", cfg.ThresholdMs())
	}

	cfg.Difficulty.Preset = Custom
	cfg.Difficulty.CustomThresholdMs = 42
	if cfg.ThresholdMs() != 42 {
		t.Errorf("expected 42ms for custom, got %d", cfg.ThresholdMs())
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Keys.Kick != "j" {
		t.Errorf("expected default kick=j, got %s", cfg.Keys.Kick)
	}

	// Verify file was created
	path := filepath.Join(tmpDir, "drum-hero", "config.toml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestLoadParsesCustom(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, "drum-hero")
	os.MkdirAll(cfgDir, 0o755)

	content := `[keys]
kick = "space"
snare = "x"
closed-hihat = "z"
open-hihat = "c"
hi-tom = "v"
mid-tom = "b"
low-tom = "n"
cymbal = "m"

[difficulty]
preset = "hard"
custom_threshold_ms = 50

[general]
songs_dir = "/tmp/songs"
`
	os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0o644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Keys.Kick != "space" {
		t.Errorf("expected kick=space, got %s", cfg.Keys.Kick)
	}
	if cfg.Difficulty.Preset != Hard {
		t.Errorf("expected hard difficulty, got %s", cfg.Difficulty.Preset)
	}
	if cfg.ThresholdMs() != 60 {
		t.Errorf("expected 60ms for hard, got %d", cfg.ThresholdMs())
	}
	if cfg.General.SongsDir != "/tmp/songs" {
		t.Errorf("expected /tmp/songs, got %s", cfg.General.SongsDir)
	}
}

func TestAllDrumTypes(t *testing.T) {
	types := AllDrumTypes()
	if len(types) != 8 {
		t.Errorf("expected 8 drum types, got %d", len(types))
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := expandHome("~/Music")
	expected := filepath.Join(home, "Music")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}

	result = expandHome("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", result)
	}
}
