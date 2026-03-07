package analysis

import (
	"math"
	"testing"
)

// generateSilenceWithImpulses creates a mono audio buffer with silence
// and sharp impulses at the given frame positions.
func generateSilenceWithImpulses(totalFrames int, impulseFrames []int) []float64 {
	mono := make([]float64, totalFrames)
	for _, f := range impulseFrames {
		if f < totalFrames {
			// Create a short burst (10 samples)
			for i := 0; i < 10 && f+i < totalFrames; i++ {
				mono[f+i] = 0.9 * math.Exp(-float64(i)*0.3)
			}
		}
	}
	return mono
}

// generateKickLike creates a low-frequency impulse (simulating a kick drum).
func generateKickLike(totalFrames, sampleRate int, atFrame int) []float64 {
	mono := make([]float64, totalFrames)
	freq := 60.0                   // Hz - kick fundamental
	decayFrames := sampleRate / 10 // 100ms decay

	for i := 0; i < decayFrames && atFrame+i < totalFrames; i++ {
		t := float64(i) / float64(sampleRate)
		envelope := math.Exp(-float64(i) / float64(decayFrames) * 5)
		mono[atFrame+i] = 0.8 * math.Sin(2*math.Pi*freq*t) * envelope
	}
	return mono
}

// generateHiHatLike creates a high-frequency noise burst (simulating a hi-hat).
func generateHiHatLike(totalFrames, sampleRate int, atFrame int) []float64 {
	mono := make([]float64, totalFrames)
	// Simple high-frequency content: sum of high sine waves with fast decay
	decayFrames := sampleRate / 50 // 20ms decay (short)

	for i := 0; i < decayFrames && atFrame+i < totalFrames; i++ {
		t := float64(i) / float64(sampleRate)
		envelope := math.Exp(-float64(i) / float64(decayFrames) * 8)
		// Mix of high frequencies
		mono[atFrame+i] = 0.5 * (math.Sin(2*math.Pi*7000*t) +
			math.Sin(2*math.Pi*9000*t) +
			math.Sin(2*math.Pi*12000*t)) / 3.0 * envelope
	}
	return mono
}

func TestDetectOnsets_SimpleImpulses(t *testing.T) {
	sampleRate := 44100
	totalFrames := sampleRate * 2 // 2 seconds

	// Place impulses at 0.5s and 1.0s
	impulseFrames := []int{22050, 44100}
	mono := generateSilenceWithImpulses(totalFrames, impulseFrames)

	onsets := DetectOnsets(mono, sampleRate)

	if len(onsets) == 0 {
		t.Fatal("expected to detect onsets, got none")
	}

	t.Logf("Detected %d onsets at frames: %v", len(onsets), onsets)

	// Onsets should be near our impulse positions (within a few hop sizes).
	// The spectral flux algorithm quantizes to hop boundaries and has latency
	// from the FFT window, so allow up to ~5 hop sizes of error.
	tolerance := defaultHopSize * 5
	for _, expected := range impulseFrames {
		found := false
		for _, onset := range onsets {
			if abs(onset-expected) < tolerance {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected onset near frame %d (tolerance %d), not found in %v", expected, tolerance, onsets)
		}
	}
}

func TestDetectOnsets_EmptySignal(t *testing.T) {
	mono := make([]float64, 44100) // 1 second of silence
	onsets := DetectOnsets(mono, 44100)
	if len(onsets) != 0 {
		t.Errorf("expected 0 onsets in silence, got %d", len(onsets))
	}
}

func TestDetectOnsets_TooShort(t *testing.T) {
	mono := make([]float64, 100) // Too short for FFT
	onsets := DetectOnsets(mono, 44100)
	if onsets != nil {
		t.Errorf("expected nil for too-short signal, got %v", onsets)
	}
}

func TestHanningWindow(t *testing.T) {
	w := hanningWindow(1024)
	if len(w) != 1024 {
		t.Errorf("expected 1024 samples, got %d", len(w))
	}

	// Window should be 0 at endpoints and 1 at center
	if w[0] > 0.001 {
		t.Errorf("expected ~0 at start, got %f", w[0])
	}
	if math.Abs(w[512]-1.0) > 0.01 {
		t.Errorf("expected ~1 at center, got %f", w[512])
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		input    []float64
		expected float64
	}{
		{[]float64{1, 2, 3}, 2},
		{[]float64{3, 1, 2}, 2},
		{[]float64{1, 2, 3, 4}, 2.5},
		{[]float64{5}, 5},
	}

	for _, tt := range tests {
		data := make([]float64, len(tt.input))
		copy(data, tt.input)
		got := median(data)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("median(%v) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestNextPow2(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{1000, 1024},
		{2048, 2048},
		{2049, 4096},
	}

	for _, tt := range tests {
		got := nextPow2(tt.input)
		if got != tt.expected {
			t.Errorf("nextPow2(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
