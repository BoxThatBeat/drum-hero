package audio

import (
	"testing"
)

func TestPlayerUnmuteWindows(t *testing.T) {
	p := &Player{
		noDrums: make([]int16, 44100*2*10), // 10 seconds stereo
		drums:   make([]int16, 44100*2*10),
		doneCh:  make(chan struct{}),
	}

	// No unmute windows — drums should be muted
	if p.isDrumUnmuted(0) {
		t.Error("expected drums muted with no unmute windows")
	}

	// Add an unmute window at 1 second (44100 * 2 samples in stereo)
	p.UnmuteDrums(1000.0, 50.0, 100.0) // 50ms before, 100ms after

	// Sample at 1 second should be unmuted
	sampleAt1s := int64(1.0 * 44100 * 2) // 1 second in stereo samples
	if !p.isDrumUnmuted(sampleAt1s) {
		t.Error("expected drums unmuted at 1 second")
	}

	// Sample at 0 should not be unmuted
	if p.isDrumUnmuted(0) {
		t.Error("expected drums muted at t=0")
	}
}

func TestPlayerCleanupUnmuteWindows(t *testing.T) {
	p := &Player{
		noDrums: make([]int16, 44100*2*10),
		drums:   make([]int16, 44100*2*10),
		doneCh:  make(chan struct{}),
	}

	// Add windows at 0.5s, 1.0s, 5.0s
	p.UnmuteDrums(500, 50, 100)
	p.UnmuteDrums(1000, 50, 100)
	p.UnmuteDrums(5000, 50, 100)

	if len(p.unmuteWindows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(p.unmuteWindows))
	}

	// Move position past the first two windows (past 1.1 seconds)
	p.position.Store(int64(1.1 * 44100 * 2))
	p.CleanupUnmuteWindows()

	p.unmuteMu.RLock()
	remaining := len(p.unmuteWindows)
	p.unmuteMu.RUnlock()

	if remaining != 1 {
		t.Errorf("expected 1 remaining window after cleanup, got %d", remaining)
	}
}

func TestPlayerDuration(t *testing.T) {
	p := &Player{
		noDrums: make([]int16, 44100*2*5), // 5 seconds stereo
	}

	dur := p.Duration()
	if dur.Seconds() < 4.9 || dur.Seconds() > 5.1 {
		t.Errorf("expected ~5s duration, got %v", dur)
	}
}

func TestPlayerCurrentTime(t *testing.T) {
	p := &Player{
		noDrums: make([]int16, 44100*2*10),
	}

	// Position at 2 seconds (2 * 44100 * 2 = 176400 interleaved samples)
	p.position.Store(176400)
	ms := p.CurrentTimeMs()
	if ms < 1990 || ms > 2010 {
		t.Errorf("expected ~2000ms, got %f", ms)
	}
}
