package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// DrumType represents a type of drum kit piece.
type DrumType string

const (
	Kick     DrumType = "kick"
	Snare    DrumType = "snare"
	ClosedHH DrumType = "closed-hihat"
	OpenHH   DrumType = "open-hihat"
	HiTom    DrumType = "hi-tom"
	MidTom   DrumType = "mid-tom"
	LowTom   DrumType = "low-tom"
	Cymbal   DrumType = "cymbal"
)

// AllDrumTypes returns all drum types in lane display order (left to right),
// matching the default key layout on a QWERTY keyboard: a s d f j k l ;
func AllDrumTypes() []DrumType {
	return []DrumType{LowTom, MidTom, HiTom, Snare, Kick, ClosedHH, OpenHH, Cymbal}
}

// Difficulty represents a named difficulty preset.
type Difficulty string

const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
	Expert Difficulty = "expert"
	Custom Difficulty = "custom"
)

// ThresholdMs returns the timing window in milliseconds for a difficulty.
func (d Difficulty) ThresholdMs() int {
	switch d {
	case Easy:
		return 150
	case Medium:
		return 100
	case Hard:
		return 60
	case Expert:
		return 30
	default:
		return 100
	}
}

// ThresholdDuration returns the timing window as a time.Duration.
func (d Difficulty) ThresholdDuration() time.Duration {
	return time.Duration(d.ThresholdMs()) * time.Millisecond
}

// KeysConfig maps drum types to keyboard keys.
type KeysConfig struct {
	Kick     string `toml:"kick"`
	Snare    string `toml:"snare"`
	ClosedHH string `toml:"closed-hihat"`
	OpenHH   string `toml:"open-hihat"`
	HiTom    string `toml:"hi-tom"`
	MidTom   string `toml:"mid-tom"`
	LowTom   string `toml:"low-tom"`
	Cymbal   string `toml:"cymbal"`
}

// DifficultyConfig holds difficulty settings.
type DifficultyConfig struct {
	Preset            Difficulty `toml:"preset"`
	CustomThresholdMs int        `toml:"custom_threshold_ms"`
}

// AudioConfig holds audio playback settings.
type AudioConfig struct {
	DrumUnmuteMs int `toml:"drum_unmute_ms"`
}

// GeneralConfig holds general application settings.
type GeneralConfig struct {
	SongsDir string `toml:"songs_dir"`
}

// Config is the top-level configuration.
type Config struct {
	Keys       KeysConfig       `toml:"keys"`
	Difficulty DifficultyConfig `toml:"difficulty"`
	Audio      AudioConfig      `toml:"audio"`
	General    GeneralConfig    `toml:"general"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Keys: KeysConfig{
			Kick:     "j",
			Snare:    "f",
			ClosedHH: "k",
			OpenHH:   "l",
			HiTom:    "d",
			MidTom:   "s",
			LowTom:   "a",
			Cymbal:   ";",
		},
		Difficulty: DifficultyConfig{
			Preset:            Medium,
			CustomThresholdMs: 80,
		},
		Audio: AudioConfig{
			DrumUnmuteMs: 300,
		},
		General: GeneralConfig{
			SongsDir: "~/Music/drum-hero",
		},
	}
}

// KeyToDrum returns a map from key string to DrumType.
func (c *Config) KeyToDrum() map[string]DrumType {
	return map[string]DrumType{
		c.Keys.Kick:     Kick,
		c.Keys.Snare:    Snare,
		c.Keys.ClosedHH: ClosedHH,
		c.Keys.OpenHH:   OpenHH,
		c.Keys.HiTom:    HiTom,
		c.Keys.MidTom:   MidTom,
		c.Keys.LowTom:   LowTom,
		c.Keys.Cymbal:   Cymbal,
	}
}

// DrumToKey returns a map from DrumType to key string.
func (c *Config) DrumToKey() map[DrumType]string {
	return map[DrumType]string{
		Kick:     c.Keys.Kick,
		Snare:    c.Keys.Snare,
		ClosedHH: c.Keys.ClosedHH,
		OpenHH:   c.Keys.OpenHH,
		HiTom:    c.Keys.HiTom,
		MidTom:   c.Keys.MidTom,
		LowTom:   c.Keys.LowTom,
		Cymbal:   c.Keys.Cymbal,
	}
}

// ThresholdMs returns the effective threshold in milliseconds.
func (c *Config) ThresholdMs() int {
	if c.Difficulty.Preset == Custom {
		return c.Difficulty.CustomThresholdMs
	}
	return c.Difficulty.Preset.ThresholdMs()
}

// ThresholdDuration returns the effective threshold as a time.Duration.
func (c *Config) ThresholdDuration() time.Duration {
	return time.Duration(c.ThresholdMs()) * time.Millisecond
}

// ExpandedSongsDir returns the songs directory with ~ expanded.
func (c *Config) ExpandedSongsDir() string {
	return expandHome(c.General.SongsDir)
}

// configDir returns the XDG config directory for drum-hero.
func configDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "drum-hero")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(configDir(), "config.toml")
}

// Load loads the config from disk, creating a default if it doesn't exist.
func Load() (Config, error) {
	cfg := DefaultConfig()
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Create default config file
		if err := writeDefault(path, cfg); err != nil {
			return cfg, fmt.Errorf("creating default config: %w", err)
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// writeDefault writes the default config to the given path.
func writeDefault(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content := `# Drum Hero Configuration

[keys]
kick = "j"
snare = "f"
closed-hihat = "k"
open-hihat = "l"
hi-tom = "d"
mid-tom = "s"
low-tom = "a"
cymbal = ";"

[difficulty]
# Options: easy (+/-150ms), medium (+/-100ms), hard (+/-60ms), expert (+/-30ms), custom
preset = "medium"
custom_threshold_ms = 80

[audio]
# How long (ms) the drum track stays audible after a correct hit
drum_unmute_ms = 300

[general]
songs_dir = "~/Music/drum-hero"
`
	return os.WriteFile(path, []byte(content), 0o644)
}

// expandHome expands ~ to the user's home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
