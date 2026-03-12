package config

import (
	"crypto/sha256"
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
	Cymbal   DrumType = "cymbal"
)

// AllDrumTypes returns all drum types in lane display order (left to right),
// matching the default key layout on a QWERTY keyboard: a s d j k
func AllDrumTypes() []DrumType {
	return []DrumType{ClosedHH, OpenHH, Snare, Kick, Cymbal}
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

// ClassifierConfig holds thresholds for drum hit classification.
// Tweak these to adjust how the classifier distinguishes drum types.
type ClassifierConfig struct {
	// KickThreshold: minimum low-frequency ratio (subBass+bass) to detect a kick (default 0.50)
	KickThreshold float64 `toml:"kick_threshold"`
	// HihatThreshold: minimum high-frequency ratio (highMid+high+veryHigh) to detect hi-hat/cymbal (default 0.20)
	HihatThreshold float64 `toml:"hihat_threshold"`
	// SnareBands: minimum number of significant frequency bands for broadband snare detection (default 4)
	SnareBands int `toml:"snare_bands"`
	// SimultaneousLow: kick low-frequency ratio for simultaneous kick+hihat detection (default 0.30)
	SimultaneousLow float64 `toml:"simultaneous_low"`
	// SimultaneousHigh: high-frequency ratio for simultaneous kick+hihat detection (default 0.15)
	SimultaneousHigh float64 `toml:"simultaneous_high"`

	// Frequency band boundaries (Hz). These define the edges between the 7 energy bands
	// used for classification: SubBass, Bass, LowMid, Mid, HighMid, High, VeryHigh.
	// The "high" bands (HighMid + High + VeryHigh) are what trigger hi-hat/cymbal detection.
	// If hi-hats aren't being detected, try lowering freq_mid to capture more energy
	// in the high bands (e.g. 1000 instead of 2000).
	FreqSubBass float64 `toml:"freq_sub_bass"` // upper edge of SubBass (default 80)
	FreqBass    float64 `toml:"freq_bass"`     // upper edge of Bass (default 200)
	FreqLowMid  float64 `toml:"freq_low_mid"`  // upper edge of LowMid (default 600)
	FreqMid     float64 `toml:"freq_mid"`      // upper edge of Mid (default 2000)
	FreqHighMid float64 `toml:"freq_high_mid"` // upper edge of HighMid (default 5000)
	FreqHigh    float64 `toml:"freq_high"`     // upper edge of High (default 10000)
	// VeryHigh captures everything above FreqHigh up to Nyquist.

	// Onset detection parameters
	OnsetFFTSize     int     `toml:"onset_fft_size"`     // FFT window size in samples (default 2048)
	OnsetHopSize     int     `toml:"onset_hop_size"`     // samples between analysis frames (default 512)
	OnsetThreshold   float64 `toml:"onset_threshold"`    // minimum spectral flux for an onset (default 0.30)
	OnsetMedianWindow int    `toml:"onset_median_window"` // adaptive threshold median window size (default 7)

	// Minimum interval (ms) between consecutive hits of each drum type.
	// Lower = allows faster rolls/double hits; higher = merges close hits into one.
	MinIntervalKickMs     int `toml:"min_interval_kick_ms"`      // default 30
	MinIntervalSnareMs    int `toml:"min_interval_snare_ms"`     // default 30
	MinIntervalClosedHHMs int `toml:"min_interval_closedhh_ms"`  // default 30
	MinIntervalOpenHHMs   int `toml:"min_interval_openhh_ms"`    // default 50
	MinIntervalCymbalMs   int `toml:"min_interval_cymbal_ms"`    // default 50
}

// MinIntervalMs returns the minimum interval for a given drum type.
func (c *ClassifierConfig) MinIntervalMs(dt DrumType) int {
	switch dt {
	case Kick:
		return c.MinIntervalKickMs
	case Snare:
		return c.MinIntervalSnareMs
	case ClosedHH:
		return c.MinIntervalClosedHHMs
	case OpenHH:
		return c.MinIntervalOpenHHMs
	case Cymbal:
		return c.MinIntervalCymbalMs
	default:
		return 30
	}
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
	Classifier ClassifierConfig `toml:"classifier"`
	General    GeneralConfig    `toml:"general"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Keys: KeysConfig{
			ClosedHH: "a",
			OpenHH:   "s",
			Snare:    "d",
			Kick:     "j",
			Cymbal:   "k",
		},
		Difficulty: DifficultyConfig{
			Preset:            Medium,
			CustomThresholdMs: 80,
		},
		Audio: AudioConfig{
			DrumUnmuteMs: 300,
		},
		Classifier: ClassifierConfig{
			KickThreshold:    0.50,
			HihatThreshold:   0.20,
			SnareBands:       4,
			SimultaneousLow:  0.30,
			SimultaneousHigh: 0.15,
			FreqSubBass:      80,
			FreqBass:         200,
			FreqLowMid:       600,
			FreqMid:          2000,
			FreqHighMid:      5000,
			FreqHigh:         10000,
			OnsetFFTSize:      2048,
			OnsetHopSize:      512,
			OnsetThreshold:    0.30,
			OnsetMedianWindow: 7,
			MinIntervalKickMs:     30,
			MinIntervalSnareMs:    30,
			MinIntervalClosedHHMs: 30,
			MinIntervalOpenHHMs:   50,
			MinIntervalCymbalMs:   50,
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
closed-hihat = "a"
open-hihat = "s"
snare = "d"
kick = "j"
cymbal = "k"

[difficulty]
# Options: easy (+/-150ms), medium (+/-100ms), hard (+/-60ms), expert (+/-30ms), custom
preset = "medium"
custom_threshold_ms = 80

[audio]
# How long (ms) the drum track stays audible after a correct hit
drum_unmute_ms = 300

[classifier]
# Thresholds for drum hit classification. Changing any value clears the cached drum map.
# Lower hihat_threshold to detect more hi-hats/cymbals; raise to detect fewer.
# Lower kick_threshold to detect more kicks; raise to require stronger bass.
# Lower snare_bands to be more lenient with snare; raise to require broader spectrum.
kick_threshold = 0.50
hihat_threshold = 0.20
snare_bands = 4
simultaneous_low = 0.30
simultaneous_high = 0.15

# Frequency band boundaries (Hz). These define the 7 energy bands used for classification:
#   SubBass [20 .. freq_sub_bass] | Bass [.. freq_bass] | LowMid [.. freq_low_mid]
#   Mid [.. freq_mid] | HighMid [.. freq_high_mid] | High [.. freq_high] | VeryHigh [.. Nyquist]
# Hi-hat/cymbal detection uses HighMid + High + VeryHigh. If hi-hats aren't detected,
# try lowering freq_mid (e.g. 1000) to shift more energy into the "high" bands.
freq_sub_bass = 80
freq_bass = 200
freq_low_mid = 600
freq_mid = 2000
freq_high_mid = 5000
freq_high = 10000

# Onset detection parameters
onset_fft_size = 2048
onset_hop_size = 512
onset_threshold = 0.30
onset_median_window = 7

# Minimum interval (ms) between consecutive hits per drum type.
# Lower = allows faster rolls/double hits. Try 20 for fast double kick.
min_interval_kick_ms = 30
min_interval_snare_ms = 30
min_interval_closedhh_ms = 30
min_interval_openhh_ms = 50
min_interval_cymbal_ms = 50

[general]
songs_dir = "~/Music/drum-hero"
`
	return os.WriteFile(path, []byte(content), 0o644)
}

// Fingerprint returns a short hash of the classifier config values.
// Used to detect when cached drum maps need re-analysis.
func (c *ClassifierConfig) Fingerprint() string {
	s := fmt.Sprintf("%.4f|%.4f|%d|%.4f|%.4f|%.1f|%.1f|%.1f|%.1f|%.1f|%.1f|%d|%d|%.4f|%d|%d|%d|%d|%d|%d",
		c.KickThreshold, c.HihatThreshold, c.SnareBands,
		c.SimultaneousLow, c.SimultaneousHigh,
		c.FreqSubBass, c.FreqBass, c.FreqLowMid,
		c.FreqMid, c.FreqHighMid, c.FreqHigh,
		c.OnsetFFTSize, c.OnsetHopSize, c.OnsetThreshold, c.OnsetMedianWindow,
		c.MinIntervalKickMs, c.MinIntervalSnareMs, c.MinIntervalClosedHHMs,
		c.MinIntervalOpenHHMs, c.MinIntervalCymbalMs)
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

// expandHome expands ~ to the user's home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
