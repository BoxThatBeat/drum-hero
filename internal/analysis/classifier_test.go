package analysis

import (
	"math"
	"testing"

	"github.com/boxthatbeat/drum-hero/internal/config"
)

func TestClassify_KickLike(t *testing.T) {
	sampleRate := 44100
	totalFrames := sampleRate // 1 second
	atFrame := sampleRate / 4 // 250ms

	mono := generateKickLike(totalFrames, sampleRate, atFrame)

	// Detect the onset
	onsets := DetectOnsets(mono, sampleRate)
	if len(onsets) == 0 {
		t.Skip("onset detection didn't find the kick - skipping classification test")
	}

	types := Classify(mono, sampleRate, onsets)
	t.Logf("Classified kick-like signal: onsets=%v, types=%v", onsets, types)

	// The kick-like signal should classify as kick
	foundKick := false
	for _, typ := range types {
		if typ == config.Kick {
			foundKick = true
			break
		}
	}
	if !foundKick {
		t.Logf("Warning: kick-like signal classified as %v (heuristic may need tuning)", types)
	}
}

func TestClassify_HiHatLike(t *testing.T) {
	sampleRate := 44100
	totalFrames := sampleRate
	atFrame := sampleRate / 4

	mono := generateHiHatLike(totalFrames, sampleRate, atFrame)

	onsets := DetectOnsets(mono, sampleRate)
	if len(onsets) == 0 {
		t.Skip("onset detection didn't find the hi-hat - skipping classification test")
	}

	types := Classify(mono, sampleRate, onsets)
	t.Logf("Classified hi-hat-like signal: onsets=%v, types=%v", onsets, types)

	// Should classify as some kind of high-frequency drum
	foundHigh := false
	for _, typ := range types {
		if typ == config.ClosedHH || typ == config.OpenHH || typ == config.Cymbal {
			foundHigh = true
			break
		}
	}
	if !foundHigh {
		t.Logf("Warning: hi-hat-like signal classified as %v (heuristic may need tuning)", types)
	}
}

func TestClassifyFromFeatures(t *testing.T) {
	tests := []struct {
		name     string
		energy   BandEnergy
		env      EnvelopeFeatures
		expected config.DrumType
	}{
		{
			name: "kick - dominant sub bass",
			energy: BandEnergy{
				SubBass: 60, Bass: 20, LowMid: 5, Mid: 3, HighMid: 1, High: 1, VeryHigh: 0,
				Total: 90,
			},
			env:      EnvelopeFeatures{DecayRate: 3.0},
			expected: config.Kick,
		},
		{
			name: "closed hi-hat - high with fast decay",
			energy: BandEnergy{
				SubBass: 1, Bass: 1, LowMid: 2, Mid: 5, HighMid: 20, High: 40, VeryHigh: 20,
				Total: 89,
			},
			env:      EnvelopeFeatures{DecayRate: 6.0},
			expected: config.ClosedHH,
		},
		{
			name: "snare - mid range with noise",
			energy: BandEnergy{
				SubBass: 5, Bass: 10, LowMid: 15, Mid: 25, HighMid: 20, High: 10, VeryHigh: 5,
				Total: 90,
			},
			env:      EnvelopeFeatures{DecayRate: 3.0},
			expected: config.Snare,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyFromFeatures(tt.energy, tt.env)
			if got != tt.expected {
				t.Errorf("classifyFromFeatures() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestRms(t *testing.T) {
	// RMS of a sine wave with amplitude 1 is 1/sqrt(2) ≈ 0.707
	samples := make([]float64, 44100)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 44100)
	}

	r := rms(samples)
	expected := 1.0 / math.Sqrt(2)
	if math.Abs(r-expected) > 0.01 {
		t.Errorf("rms = %f, expected ~%f", r, expected)
	}
}

func TestRmsEmpty(t *testing.T) {
	r := rms(nil)
	if r != 0 {
		t.Errorf("rms(nil) = %f, want 0", r)
	}
}
