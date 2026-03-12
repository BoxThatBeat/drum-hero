package analysis

import (
	"math"
	"math/cmplx"

	"github.com/boxthatbeat/drum-hero/internal/config"
	"github.com/madelynnblue/go-dsp/fft"
)

// rawOnsetMinIntervalMs is a very small global minimum between raw onsets.
// Per-drum-type intervals are applied after classification in the drum map builder.
const rawOnsetMinIntervalMs = 10

// DetectOnsets finds onset times (in sample frames) from mono audio using spectral flux.
// Uses parameters from the classifier config.
func DetectOnsets(mono []float64, sampleRate int, cfg config.ClassifierConfig) []int {
	fftSize := cfg.OnsetFFTSize
	hopSize := cfg.OnsetHopSize
	threshold := cfg.OnsetThreshold
	medianWindow := cfg.OnsetMedianWindow

	if len(mono) < fftSize {
		return nil
	}

	// Compute spectral flux
	flux := spectralFlux(mono, fftSize, hopSize)

	// Adaptive thresholding: use a moving median + offset
	adaptiveThresh := adaptiveThreshold(flux, medianWindow, threshold)

	// Peak picking: find local maxima above threshold
	// Use a small global min interval — per-type intervals are applied post-classification.
	minIntervalFrames := int(float64(rawOnsetMinIntervalMs) / 1000.0 * float64(sampleRate) / float64(hopSize))
	if minIntervalFrames < 1 {
		minIntervalFrames = 1
	}

	var onsets []int
	lastOnset := -minIntervalFrames // Allow first onset at any position

	for i := 1; i < len(flux)-1; i++ {
		// Must be above adaptive threshold
		if flux[i] <= adaptiveThresh[i] {
			continue
		}
		// Must be a local maximum
		if flux[i] <= flux[i-1] || flux[i] <= flux[i+1] {
			continue
		}
		// Must be far enough from last onset
		if i-lastOnset < minIntervalFrames {
			continue
		}

		// Convert flux frame index to sample frame
		sampleFrame := i * hopSize
		onsets = append(onsets, sampleFrame)
		lastOnset = i
	}

	return onsets
}

// spectralFlux computes the positive spectral flux over time.
func spectralFlux(mono []float64, fftSize, hopSize int) []float64 {
	numFrames := (len(mono) - fftSize) / hopSize
	if numFrames <= 0 {
		return nil
	}

	flux := make([]float64, numFrames)
	prevMag := make([]float64, fftSize/2+1)

	window := hanningWindow(fftSize)

	for i := 0; i < numFrames; i++ {
		start := i * hopSize
		frame := make([]float64, fftSize)
		copy(frame, mono[start:start+fftSize])

		// Apply window
		for j := range frame {
			frame[j] *= window[j]
		}

		// FFT
		spectrum := fft.FFTReal(frame)

		// Compute magnitude spectrum (only positive frequencies)
		mag := make([]float64, fftSize/2+1)
		for j := 0; j <= fftSize/2; j++ {
			mag[j] = cmplx.Abs(spectrum[j])
		}

		// Compute positive spectral flux (only increases in energy)
		var f float64
		for j := range mag {
			diff := mag[j] - prevMag[j]
			if diff > 0 {
				f += diff
			}
		}
		flux[i] = f

		copy(prevMag, mag)
	}

	// Normalize flux
	maxFlux := 0.0
	for _, f := range flux {
		if f > maxFlux {
			maxFlux = f
		}
	}
	if maxFlux > 0 {
		for i := range flux {
			flux[i] /= maxFlux
		}
	}

	return flux
}

// adaptiveThreshold computes an adaptive threshold using a moving median + offset.
func adaptiveThreshold(flux []float64, windowSize int, offset float64) []float64 {
	thresh := make([]float64, len(flux))
	half := windowSize / 2

	for i := range flux {
		start := i - half
		if start < 0 {
			start = 0
		}
		end := i + half + 1
		if end > len(flux) {
			end = len(flux)
		}

		// Compute median of window
		window := make([]float64, end-start)
		copy(window, flux[start:end])
		med := median(window)
		thresh[i] = med + offset
	}

	return thresh
}

// median returns the median of a slice (modifies the slice order).
func median(data []float64) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}

	// Simple insertion sort for small windows
	for i := 1; i < n; i++ {
		key := data[i]
		j := i - 1
		for j >= 0 && data[j] > key {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}

	if n%2 == 0 {
		return (data[n/2-1] + data[n/2]) / 2
	}
	return data[n/2]
}

// hanningWindow generates a Hanning window of the given size.
func hanningWindow(size int) []float64 {
	w := make([]float64, size)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(size-1)))
	}
	return w
}
