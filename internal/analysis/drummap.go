package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/boxthatbeat/drum-hero/internal/audio"
	"github.com/boxthatbeat/drum-hero/internal/cache"
	"github.com/boxthatbeat/drum-hero/internal/config"
)

// DrumHit represents a single detected drum hit with its time and type.
type DrumHit struct {
	// TimeMs is the time of the hit in milliseconds from the start of the song.
	TimeMs float64 `json:"time_ms"`
	// Type is the classified drum type.
	Type config.DrumType `json:"type"`
}

// DrumMap is a list of drum hits for a song.
type DrumMap struct {
	Hits                []DrumHit `json:"hits"`
	SampleRate          int       `json:"sample_rate"`
	DurationMs          float64   `json:"duration_ms"`
	ClassifierFingerprint string  `json:"classifier_fingerprint,omitempty"`
}

// Duration returns the total duration of the drum map.
func (dm *DrumMap) Duration() time.Duration {
	return time.Duration(dm.DurationMs * float64(time.Millisecond))
}

// HitsInWindow returns all hits within [startMs, endMs).
func (dm *DrumMap) HitsInWindow(startMs, endMs float64) []DrumHit {
	var result []DrumHit
	for _, h := range dm.Hits {
		if h.TimeMs >= startMs && h.TimeMs < endMs {
			result = append(result, h)
		}
	}
	return result
}

// Analyze detects drum hits from a separated drum track and classifies them.
// If a cached drum map exists for the given hash with matching classifier config, it is loaded instead.
func Analyze(hash string, cfg config.ClassifierConfig, onProgress audio.ProgressFunc) (*DrumMap, error) {
	if onProgress == nil {
		onProgress = func(string) {}
	}

	fingerprint := cfg.Fingerprint()

	// Check for cached drum map with matching classifier config
	if cache.HasDrumMap(hash) {
		dm, err := LoadDrumMap(cache.DrumMapPath(hash))
		if err == nil && dm.ClassifierFingerprint == fingerprint {
			onProgress("Loading cached drum map...")
			return dm, nil
		}
		// Config changed — re-analyze
		onProgress("Classifier settings changed, re-analyzing...")
	}

	// Load the separated drums track
	drumsPath := cache.DrumsPath(hash)
	onProgress("Loading drum track...")
	audioData, err := audio.DecodeWAV(drumsPath)
	if err != nil {
		return nil, fmt.Errorf("decoding drums wav: %w", err)
	}

	// Detect onsets
	onProgress("Detecting drum hits...")
	onsets := DetectOnsets(audioData.Mono, audioData.SampleRate, cfg)

	// Classify each onset (may return multiple types per onset for simultaneous hits)
	onProgress(fmt.Sprintf("Classifying %d drum hits...", len(onsets)))
	typeSets := Classify(audioData.Mono, audioData.SampleRate, onsets, cfg)

	// Build drum map — expand multi-type onsets into separate DrumHit entries
	var allHits []DrumHit
	for i, onset := range onsets {
		timeMs := float64(onset) / float64(audioData.SampleRate) * 1000.0
		for _, dt := range typeSets[i] {
			allHits = append(allHits, DrumHit{
				TimeMs: timeMs,
				Type:   dt,
			})
		}
	}

	// Apply per-drum-type minimum interval filtering.
	// For each drum type, drop hits that are too close to the previous hit of the same type.
	lastHitTime := make(map[config.DrumType]float64)
	var hits []DrumHit
	for _, h := range allHits {
		minMs := float64(cfg.MinIntervalMs(h.Type))
		if prev, ok := lastHitTime[h.Type]; ok && h.TimeMs-prev < minMs {
			continue
		}
		hits = append(hits, h)
		lastHitTime[h.Type] = h.TimeMs
	}

	dm := &DrumMap{
		Hits:                  hits,
		SampleRate:            audioData.SampleRate,
		DurationMs:            audioData.Duration.Seconds() * 1000.0,
		ClassifierFingerprint: fingerprint,
	}

	// Cache the drum map
	onProgress("Caching drum map...")
	if err := SaveDrumMap(cache.DrumMapPath(hash), dm); err != nil {
		return nil, fmt.Errorf("caching drum map: %w", err)
	}

	onProgress(fmt.Sprintf("Analysis complete: %d drum hits detected", len(hits)))
	return dm, nil
}

// SaveDrumMap saves a DrumMap to a JSON file.
func SaveDrumMap(path string, dm *DrumMap) error {
	data, err := json.MarshalIndent(dm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling drum map: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadDrumMap loads a DrumMap from a JSON file.
func LoadDrumMap(path string) (*DrumMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading drum map: %w", err)
	}

	var dm DrumMap
	if err := json.Unmarshal(data, &dm); err != nil {
		return nil, fmt.Errorf("parsing drum map: %w", err)
	}

	return &dm, nil
}
