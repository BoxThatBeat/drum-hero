package analysis

import (
	"math"
	"math/cmplx"
	"sort"

	"github.com/boxthatbeat/drum-hero/internal/config"
	"github.com/boxthatbeat/drum-hero/internal/logger"
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
func Classify(mono []float64, sampleRate int, onsets []int, cfg config.ClassifierConfig) [][]config.DrumType {
	if len(onsets) == 0 {
		return nil
	}

	windowSamples := int(float64(classifyWindowMs) / 1000.0 * float64(sampleRate))
	fftSize := nextPow2(windowSamples)
	window := hanningWindow(fftSize)

	// Pre-compute spectra for all onsets
	spectra := make([][]complex128, len(onsets))
	for i, onset := range onsets {
		frame := extractFrame(mono, onset, fftSize)
		for j := range frame {
			frame[j] *= window[j]
		}
		spectra[i] = fft.FFTReal(frame)
	}

	// Adaptive calibration: analyze spectral centroids to find per-song boundaries
	if cfg.AdaptiveCalibration {
		cfg = calibrateBands(spectra, fftSize, sampleRate, cfg)
	}

	// Classify each onset
	types := make([][]config.DrumType, len(onsets))
	for i, onset := range onsets {
		energy := computeBandEnergy(spectra[i], fftSize, sampleRate, cfg)
		env := computeEnvelope(mono, onset, sampleRate)
		types[i] = classifyFromFeatures(energy, env, cfg)
	}

	return types
}

// spectralCentroid computes the weighted average frequency of a spectrum.
func spectralCentroid(spectrum []complex128, fftSize, sampleRate int) float64 {
	freqRes := float64(sampleRate) / float64(fftSize)
	var weightedSum, totalMag float64

	for bin := 1; bin <= fftSize/2; bin++ {
		freq := float64(bin) * freqRes
		mag := cmplx.Abs(spectrum[bin])
		weightedSum += freq * mag
		totalMag += mag
	}

	if totalMag == 0 {
		return 0
	}
	return weightedSum / totalMag
}

// calibrateBands analyzes all onsets' spectral centroids to find natural frequency
// boundaries for this specific song. Returns a modified config with adjusted freq_* values.
func calibrateBands(spectra [][]complex128, fftSize, sampleRate int, cfg config.ClassifierConfig) config.ClassifierConfig {
	if len(spectra) < 10 {
		// Not enough onsets to calibrate reliably
		return cfg
	}

	// Compute spectral centroid for each onset
	centroids := make([]float64, len(spectra))
	for i, spec := range spectra {
		centroids[i] = spectralCentroid(spec, fftSize, sampleRate)
	}

	// Sort centroids to find distribution
	sorted := make([]float64, len(centroids))
	copy(sorted, centroids)
	sort.Float64s(sorted)

	// Find natural clusters using percentile-based splits.
	// Drums typically cluster into 3 groups: low (kick), mid (snare), high (hihat/cymbal).
	// Use the gaps between percentile ranges to find boundaries.

	// p25 = rough kick/snare boundary area, p65 = rough snare/hihat boundary area
	p25 := sorted[len(sorted)*25/100]
	p50 := sorted[len(sorted)*50/100]
	p65 := sorted[len(sorted)*65/100]
	p85 := sorted[len(sorted)*85/100]

	// Find the largest gap in the sorted centroids around the snare/hihat boundary.
	// Search between p50 and p85 for the biggest jump — that's likely where
	// the snare-dominant region ends and hi-hat region begins.
	bestGap := 0.0
	bestGapFreq := 0.0
	searchStart := len(sorted) * 40 / 100
	searchEnd := len(sorted) * 90 / 100
	if searchEnd >= len(sorted) {
		searchEnd = len(sorted) - 1
	}
	for i := searchStart; i < searchEnd; i++ {
		gap := sorted[i+1] - sorted[i]
		if gap > bestGap {
			bestGap = gap
			bestGapFreq = (sorted[i] + sorted[i+1]) / 2
		}
	}

	// Similarly find the kick/snare boundary gap in the lower range
	bestLowGap := 0.0
	bestLowGapFreq := 0.0
	lowSearchEnd := len(sorted) * 50 / 100
	for i := 0; i < lowSearchEnd; i++ {
		gap := sorted[i+1] - sorted[i]
		if gap > bestLowGap {
			bestLowGap = gap
			bestLowGapFreq = (sorted[i] + sorted[i+1]) / 2
		}
	}

	logger.Log("[calibrate] %d onsets, centroid range: %.0f-%.0f Hz", len(centroids), sorted[0], sorted[len(sorted)-1])
	logger.Log("[calibrate] percentiles: p25=%.0f p50=%.0f p65=%.0f p85=%.0f", p25, p50, p65, p85)
	logger.Log("[calibrate] best mid/high gap: %.0f Hz (gap size: %.0f Hz)", bestGapFreq, bestGap)
	logger.Log("[calibrate] best low/mid gap: %.0f Hz (gap size: %.0f Hz)", bestLowGapFreq, bestLowGap)

	// Adjust freq_mid (the critical snare/hihat boundary) if we found a meaningful gap.
	// A "meaningful" gap is at least 100 Hz wide — otherwise the distribution is too uniform.
	if bestGap > 100 && bestGapFreq > 200 && bestGapFreq < float64(sampleRate)/2 {
		logger.Log("[calibrate] adjusting freq_mid: %.0f -> %.0f Hz", cfg.FreqMid, bestGapFreq)
		cfg.FreqMid = bestGapFreq
	}

	// Adjust freq_low_mid (kick/snare boundary) if we found a meaningful low gap
	if bestLowGap > 50 && bestLowGapFreq > 50 && bestLowGapFreq < cfg.FreqMid {
		logger.Log("[calibrate] adjusting freq_low_mid: %.0f -> %.0f Hz", cfg.FreqLowMid, bestLowGapFreq)
		cfg.FreqLowMid = bestLowGapFreq
	}

	// Adjust freq_high_mid based on where the high-centroid onsets actually cluster
	if p85 > cfg.FreqMid && p85 < float64(sampleRate)/2 {
		// Set high_mid boundary between the mid/high gap and the p85
		highMid := (bestGapFreq + p85) / 2
		if highMid > cfg.FreqMid && highMid < cfg.FreqHigh {
			logger.Log("[calibrate] adjusting freq_high_mid: %.0f -> %.0f Hz", cfg.FreqHighMid, highMid)
			cfg.FreqHighMid = highMid
		}
	}

	return cfg
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
func computeBandEnergy(spectrum []complex128, fftSize, sampleRate int, cfg config.ClassifierConfig) BandEnergy {
	freqRes := float64(sampleRate) / float64(fftSize) // Hz per bin
	var energy BandEnergy

	for bin := 0; bin <= fftSize/2; bin++ {
		freq := float64(bin) * freqRes
		mag := cmplx.Abs(spectrum[bin])
		power := mag * mag

		switch {
		case freq >= 20 && freq < cfg.FreqSubBass:
			energy.SubBass += power
		case freq >= cfg.FreqSubBass && freq < cfg.FreqBass:
			energy.Bass += power
		case freq >= cfg.FreqBass && freq < cfg.FreqLowMid:
			energy.LowMid += power
		case freq >= cfg.FreqLowMid && freq < cfg.FreqMid:
			energy.Mid += power
		case freq >= cfg.FreqMid && freq < cfg.FreqHighMid:
			energy.HighMid += power
		case freq >= cfg.FreqHighMid && freq < cfg.FreqHigh:
			energy.High += power
		case freq >= cfg.FreqHigh:
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
// Only classifies into 5 types: Kick, Snare, ClosedHH, OpenHH, Cymbal.
func classifyFromFeatures(energy BandEnergy, env EnvelopeFeatures, cfg config.ClassifierConfig) []config.DrumType {
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

	_ = bassRatio // used in ratios above

	// --- Simultaneous hit detection ---
	// When a kick and hi-hat/cymbal play together, the onset has both
	// strong sub-bass AND strong high-frequency energy.
	hasKick := lowRatio > cfg.SimultaneousLow && subBassRatio > cfg.SimultaneousLow*0.33
	hasHighFreq := allHighRatio > cfg.SimultaneousHigh

	if hasKick && hasHighFreq {
		high := classifyHighFreq(energy, env)
		return []config.DrumType{config.Kick, high}
	}

	// --- Single instrument classification ---

	// 1. Kick: dominant low frequency energy
	if lowRatio > cfg.KickThreshold && subBassRatio > cfg.KickThreshold*0.3 {
		return []config.DrumType{config.Kick}
	}
	// Secondary kick detection for deeper kicks
	if lowRatio > cfg.KickThreshold*0.8 && subBassRatio > cfg.KickThreshold*0.24 && allHighRatio < 0.1 {
		return []config.DrumType{config.Kick}
	}

	// 2. Snare: broadband energy spread (energy across many bands).
	// Snare wires produce noise across the full spectrum. A snare has significant
	// energy in low-mid (body) AND mid/high-mid (wire buzz), unlike hi-hats which
	// are concentrated in high frequencies only.
	// Check snare BEFORE hi-hat because snares have high-frequency content too.
	significantBands := countSignificantBands(energy)
	hasMidBody := lowMidRatio > 0.05 || midRatio > 0.1

	if significantBands >= cfg.SnareBands && hasMidBody {
		return []config.DrumType{config.Snare}
	}

	// 3. Hi-hat / Cymbal: high frequency content, concentrated in upper bands
	if allHighRatio > cfg.HihatThreshold {
		return []config.DrumType{classifyHighFreq(energy, env)}
	}

	// 4. Snare fallback: anything with mid-range content
	if lowMidRatio > 0.05 || midRatio > 0.05 || highMidRatio > 0.03 {
		return []config.DrumType{config.Snare}
	}

	// 5. Fallbacks
	if lowRatio > cfg.KickThreshold*0.6 {
		return []config.DrumType{config.Kick}
	}
	if highRatio > cfg.HihatThreshold*0.5 {
		return []config.DrumType{config.ClosedHH}
	}

	return []config.DrumType{config.Snare}
}

// classifyHighFreq distinguishes between closed hi-hat, open hi-hat, and cymbal
// based on the high-frequency energy distribution and envelope.
func classifyHighFreq(energy BandEnergy, env EnvelopeFeatures) config.DrumType {
	// Fast decay = closed hi-hat (short, tight sound)
	if env.DecayRate > 2.5 {
		return config.ClosedHH
	}

	// More very-high shimmer = cymbal (sustained, bright)
	if energy.VeryHigh > energy.High*0.8 {
		if env.DecayRate < 1.5 {
			return config.Cymbal
		}
		return config.OpenHH
	}

	// Slow decay = open hi-hat (ringing)
	if env.DecayRate < 1.8 {
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
