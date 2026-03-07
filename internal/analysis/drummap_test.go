package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boxthatbeat/drum-hero/internal/config"
)

func TestDrumMapSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "drummap.json")

	dm := &DrumMap{
		Hits: []DrumHit{
			{TimeMs: 100.0, Type: config.Kick},
			{TimeMs: 350.5, Type: config.Snare},
			{TimeMs: 600.0, Type: config.ClosedHH},
		},
		SampleRate: 44100,
		DurationMs: 10000.0,
	}

	if err := SaveDrumMap(path, dm); err != nil {
		t.Fatalf("SaveDrumMap() error: %v", err)
	}

	loaded, err := LoadDrumMap(path)
	if err != nil {
		t.Fatalf("LoadDrumMap() error: %v", err)
	}

	if len(loaded.Hits) != len(dm.Hits) {
		t.Fatalf("expected %d hits, got %d", len(dm.Hits), len(loaded.Hits))
	}

	for i, hit := range loaded.Hits {
		if hit.TimeMs != dm.Hits[i].TimeMs {
			t.Errorf("hit %d: expected TimeMs %f, got %f", i, dm.Hits[i].TimeMs, hit.TimeMs)
		}
		if hit.Type != dm.Hits[i].Type {
			t.Errorf("hit %d: expected Type %s, got %s", i, dm.Hits[i].Type, hit.Type)
		}
	}

	if loaded.SampleRate != dm.SampleRate {
		t.Errorf("expected sample rate %d, got %d", dm.SampleRate, loaded.SampleRate)
	}
}

func TestDrumMapHitsInWindow(t *testing.T) {
	dm := &DrumMap{
		Hits: []DrumHit{
			{TimeMs: 100, Type: config.Kick},
			{TimeMs: 200, Type: config.Snare},
			{TimeMs: 300, Type: config.ClosedHH},
			{TimeMs: 400, Type: config.Kick},
			{TimeMs: 500, Type: config.Snare},
		},
	}

	hits := dm.HitsInWindow(150, 350)
	if len(hits) != 2 {
		t.Errorf("expected 2 hits in window [150, 350), got %d", len(hits))
	}

	hits = dm.HitsInWindow(0, 600)
	if len(hits) != 5 {
		t.Errorf("expected 5 hits in window [0, 600), got %d", len(hits))
	}

	hits = dm.HitsInWindow(600, 1000)
	if len(hits) != 0 {
		t.Errorf("expected 0 hits in window [600, 1000), got %d", len(hits))
	}
}

func TestDrumMapDuration(t *testing.T) {
	dm := &DrumMap{DurationMs: 5000}
	dur := dm.Duration()
	if dur.Seconds() < 4.9 || dur.Seconds() > 5.1 {
		t.Errorf("expected ~5s duration, got %v", dur)
	}
}

func TestLoadDrumMapNotFound(t *testing.T) {
	_, err := LoadDrumMap("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSaveDrumMapDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Verify we can save to a valid path
	path := filepath.Join(tmpDir, "sub", "drummap.json")
	os.MkdirAll(filepath.Dir(path), 0o755)

	dm := &DrumMap{Hits: []DrumHit{}, SampleRate: 44100, DurationMs: 0}
	if err := SaveDrumMap(path, dm); err != nil {
		t.Fatalf("SaveDrumMap() error: %v", err)
	}
}
