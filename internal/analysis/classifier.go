package analysis

import (
	"math"
	"math/cmplx"

	"github.com/boxthatbeat/drum-hero/internal/config"
	"github.com/madelynnblue/go-dsp/fft"
)

const (
	// classifyWindowMs is the window around each onset to analyze for classification.
	classifyWindowMs = 50
)

// BandEnergy holds energy in different frequency bands.
type BandEnergy struct {
	SubBass  float64 // 20-80 Hz (kick fundamental)
	Bass     float64 // 80-200 Hz (kick body, low tom)
	LowMid   float64 // 200-600 Hz (toms, snare body)
	Mid      float64 // 600-2000 Hz (snare ring)
	HighMid  float64 // 2000-5000 Hz (snare sizzle)
	High     float64 // 5000-10000 Hz (hi-hat, cymbal)
	VeryHigh float64 // 10000-20000 Hz (hi-hat air, cymbal shimmer)
	Total    float64
}

// Classify determines the drum type(s) for each onset based on spectral analysis.
// Returns a slice of DrumType slices — each onset may produce multiple simultaneous types.
func Classify(mono []float64, sampleRate int, onsets []int) [][]config.DrumType {
	types := make([][]config.DrumType, len(onsets))
	windowSamples := int(float64(classifyWindowMs) / 1000.0 * float64(sampleRate))

	// Use next power of 2 for FFT
	fftSize := nextPow2(windowSamples)

	window := hanningWindow(fftSize)

	for i, onset := range onsets {
		// Extract window around onset
		frame := extractFrame(mono, onset, fftSize)

		// Apply window
		for j := range frame {
			frame[j] *= window[j]
		}

		// Compute FFT
		spectrum := fft.FFTReal(frame)

		// Compute band energies
		energy := computeBandEnergy(spectrum, fftSize, sampleRate)

		// Compute temporal envelope features
		env := computeEnvelope(mono, onset, sampleRate)

		// Classify based on band energy ratios and envelope
		types[i] = classifyFromFeatures(energy, env)
	}

	return types
}

// extractFrame extracts a window of samples starting at the given frame index.
func extractFrame(mono []float64, start, size int) []float64 {
	frame := make([]float64, size)
	for i := 0; i < size; i++ {
		idx := start + i
		if idx >= 0 && idx < len(mono) {
			frame[i] = mono[idx]
		}
	}
	return frame
}

// computeBandEnergy computes the energy in different frequency bands from an FFT spectrum.
func computeBandEnergy(spectrum []complex128, fftSize, sampleRate int) BandEnergy {
	freqRes := float64(sampleRate) / float64(fftSize) // Hz per bin
	var energy BandEnergy

	for bin := 0; bin <= fftSize/2; bin++ {
		freq := float64(bin) * freqRes
		mag := cmplx.Abs(spectrum[bin])
		power := mag * mag

		switch {
		case freq >= 20 && freq < 80:
			energy.SubBass += power
		case freq >= 80 && freq < 200:
			energy.Bass += power
		case freq >= 200 && freq < 600:
			energy.LowMid += power
		case freq >= 600 && freq < 2000:
			energy.Mid += power
		case freq >= 2000 && freq < 5000:
			energy.HighMid += power
		case freq >= 5000 && freq < 10000:
			energy.High += power
		case freq >= 10000:
			energy.VeryHigh += power
		}
	}

	energy.Total = energy.SubBass + energy.Bass + energy.LowMid +
		energy.Mid + energy.HighMid + energy.High + energy.VeryHigh

	return energy
}

// EnvelopeFeatures holds temporal characteristics of a drum hit.
type EnvelopeFeatures struct {
	// DecayRate: how quickly the energy drops (higher = faster decay)
	DecayRate float64
	// AttackSharpness: ratio of peak to average in attack window
	AttackSharpness float64
}

// computeEnvelope analyzes the temporal envelope around an onset.
func computeEnvelope(mono []float64, onset, sampleRate int) EnvelopeFeatures {
	// Analyze 100ms after onset for decay
	decayWindow := sampleRate / 10 // 100ms
	if onset+decayWindow > len(mono) {
		decayWindow = len(mono) - onset
	}
	if decayWindow <= 0 {
		return EnvelopeFeatures{}
	}

	// RMS in first 10ms vs RMS in last 50ms of window
	shortWindow := sampleRate / 100 // 10ms
	if shortWindow > decayWindow {
		shortWindow = decayWindow
	}

	attackRMS := rms(mono[onset : onset+shortWindow])

	tailStart := onset + decayWindow/2
	tailEnd := onset + decayWindow
	if tailStart >= len(mono) {
		tailStart = len(mono) - 1
	}
	if tailEnd > len(mono) {
		tailEnd = len(mono)
	}

	tailRMS := rms(mono[tailStart:tailEnd])

	decayRate := 0.0
	if tailRMS > 0 {
		decayRate = attackRMS / tailRMS
	}

	return EnvelopeFeatures{
		DecayRate:       decayRate,
		AttackSharpness: attackRMS,
	}
}

// classifyFromFeatures classifies a drum hit based on spectral band energies and envelope.
// Returns one or more drum types for simultaneous hits (e.g. kick + hi-hat).
func classifyFromFeatures(energy BandEnergy, env EnvelopeFeatures) []config.DrumType {
	if energy.Total == 0 {
		return []config.DrumType{config.Snare} // fallback
	}

	// Normalize band energies to ratios
	subBassRatio := energy.SubBass / energy.Total
	bassRatio := energy.Bass / energy.Total
	lowMidRatio := energy.LowMid / energy.Total
	midRatio := energy.Mid / energy.Total
	highMidRatio := energy.HighMid / energy.Total
	highRatio := (energy.High + energy.VeryHigh) / energy.Total

	lowRatio := subBassRatio + bassRatio
	allHighRatio := highRatio + highMidRatio

	// --- Simultaneous hit detection ---
	// When a kick and hi-hat/cymbal play together, the onset has both
	// strong sub-bass AND strong high-frequency energy.
	hasKick := lowRatio > 0.3 && subBassRatio > 0.1
	hasHighFreq := allHighRatio > 0.15

	if hasKick && hasHighFreq {
		high := classifyHighFreq(energy, env)
		return []config.DrumType{config.Kick, high}
	}

	// --- Single instrument classification ---

	// 1. Kick: dominant low frequency energy
	if lowRatio > 0.5 && subBassRatio > 0.15 {
		return []config.DrumType{config.Kick}
	}
	// Secondary kick detection for deeper kicks
	if lowRatio > 0.4 && subBassRatio > 0.12 && allHighRatio < 0.15 {
		return []config.DrumType{config.Kick}
	}

	// 2. Hi-hat / Cymbal: dominant high frequency energy (lowered threshold)
	if allHighRatio > 0.25 {
		return []config.DrumType{classifyHighFreq(energy, env)}
	}

	// 3. Snare: broadband spectrum with mid-range content
	// Snare wires create noise across mid and high-mid bands, distinguishing
	// snare from toms which have concentrated low-mid energy.
	//
	// Key insight: snares have significant energy above 2kHz (highMid+high),
	// while toms do not. Even a small amount of high-frequency content
	// alongside low-mid body resonance indicates snare.
	hasSnareNoise := highMidRatio > 0.05 || highRatio > 0.05
	hasSnareBody := lowMidRatio > 0.08 || midRatio > 0.1

	if hasSnareNoise && hasSnareBody {
		return []config.DrumType{config.Snare}
	}

	// Snare with strong mid-range
	if midRatio+highMidRatio > 0.2 && lowMidRatio > 0.08 {
		return []config.DrumType{config.Snare}
	}

	// Broadband energy spread = snare (energy in 4+ bands)
	significantBands := countSignificantBands(energy)
	if significantBands >= 4 && midRatio > 0.08 {
		return []config.DrumType{config.Snare}
	}

	// 4. Toms: low-mid dominant with NO significant high-frequency content
	if lowMidRatio > 0.2 {
		// Only classify as tom if there's truly no high-frequency content
		if bassRatio > lowMidRatio*1.3 {
			return []config.DrumType{config.LowTom}
		}
		if lowMidRatio > midRatio*1.5 {
			return []config.DrumType{config.MidTom}
		}
		return []config.DrumType{config.HiTom}
	}

	// 5. Fallbacks
	if lowRatio > 0.35 {
		return []config.DrumType{config.Kick}
	}
	if highRatio > 0.15 {
		return []config.DrumType{config.ClosedHH}
	}

	return []config.DrumType{config.Snare}
}

// classifyHighFreq distinguishes between closed hi-hat, open hi-hat, and cymbal
// based on the high-frequency energy distribution and envelope.
func classifyHighFreq(energy BandEnergy, env EnvelopeFeatures) config.DrumType {
	// Fast decay = closed hi-hat (short, tight sound)
	if env.DecayRate > 3.0 {
		return config.ClosedHH
	}

	// More very-high shimmer = cymbal
	if energy.VeryHigh > energy.High {
		if env.DecayRate < 1.5 {
			return config.Cymbal
		}
		return config.OpenHH
	}

	// Moderate decay = open hi-hat
	if env.DecayRate < 2.0 {
		return config.OpenHH
	}

	return config.ClosedHH
}

// countSignificantBands counts how many frequency bands have > 5% of total energy.
// A high count indicates broadband content (typical of snare).
func countSignificantBands(energy BandEnergy) int {
	threshold := energy.Total * 0.05
	count := 0
	if energy.SubBass > threshold {
		count++
	}
	if energy.Bass > threshold {
		count++
	}
	if energy.LowMid > threshold {
		count++
	}
	if energy.Mid > threshold {
		count++
	}
	if energy.HighMid > threshold {
		count++
	}
	if energy.High > threshold {
		count++
	}
	if energy.VeryHigh > threshold {
		count++
	}
	return count
}

// rms computes root mean square of a signal.
func rms(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += s * s
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// nextPow2 returns the next power of 2 >= n.
func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
