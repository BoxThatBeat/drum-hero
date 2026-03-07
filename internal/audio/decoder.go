package audio

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/go-audio/wav"
)

// AudioData holds decoded audio data as float64 samples normalized to [-1.0, 1.0].
type AudioData struct {
	// Samples holds interleaved samples for all channels.
	Samples []float64
	// Mono holds a mono mixdown of the audio.
	Mono []float64
	// SampleRate is the sample rate in Hz.
	SampleRate int
	// Channels is the number of audio channels.
	Channels int
	// Duration is the total duration of the audio.
	Duration time.Duration
}

// NumFrames returns the number of sample frames (samples per channel).
func (a *AudioData) NumFrames() int {
	if a.Channels == 0 {
		return 0
	}
	return len(a.Samples) / a.Channels
}

// FrameAtTime returns the sample frame index for a given time offset.
func (a *AudioData) FrameAtTime(t time.Duration) int {
	frame := int(float64(t.Nanoseconds()) * float64(a.SampleRate) / 1e9)
	if frame < 0 {
		return 0
	}
	if frame >= a.NumFrames() {
		return a.NumFrames() - 1
	}
	return frame
}

// TimeAtFrame returns the time offset for a given sample frame index.
func (a *AudioData) TimeAtFrame(frame int) time.Duration {
	return time.Duration(float64(frame) / float64(a.SampleRate) * float64(time.Second))
}

// DecodeWAV decodes a WAV file into AudioData with float64 samples.
func DecodeWAV(path string) (*AudioData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening wav file: %w", err)
	}
	defer f.Close()

	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		return nil, fmt.Errorf("invalid wav file: %s", path)
	}

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("decoding wav: %w", err)
	}

	channels := int(dec.NumChans)
	sampleRate := int(dec.SampleRate)
	bitDepth := int(dec.BitDepth)

	// Determine the max value for normalization based on bit depth
	var maxVal float64
	switch bitDepth {
	case 8:
		maxVal = math.MaxInt8
	case 16:
		maxVal = math.MaxInt16
	case 24:
		maxVal = float64(1<<23 - 1)
	case 32:
		maxVal = math.MaxInt32
	default:
		maxVal = math.MaxInt16
	}

	// Convert int samples to float64 normalized to [-1.0, 1.0]
	samples := make([]float64, len(buf.Data))
	for i, s := range buf.Data {
		samples[i] = float64(s) / maxVal
	}

	// Create mono mixdown
	numFrames := len(samples) / channels
	mono := make([]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		var sum float64
		for ch := 0; ch < channels; ch++ {
			sum += samples[i*channels+ch]
		}
		mono[i] = sum / float64(channels)
	}

	duration := time.Duration(float64(numFrames) / float64(sampleRate) * float64(time.Second))

	return &AudioData{
		Samples:    samples,
		Mono:       mono,
		SampleRate: sampleRate,
		Channels:   channels,
		Duration:   duration,
	}, nil
}

// LoadRawSamples loads a WAV file and returns the raw int16 interleaved PCM data
// suitable for audio playback. Returns samples, channels, sampleRate.
func LoadRawSamples(path string) ([]int16, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("opening wav file: %w", err)
	}
	defer f.Close()

	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		return nil, 0, 0, fmt.Errorf("invalid wav file: %s", path)
	}

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decoding wav: %w", err)
	}

	channels := int(dec.NumChans)
	sampleRate := int(dec.SampleRate)
	bitDepth := int(dec.BitDepth)

	// Convert to int16 for playback
	samples := make([]int16, len(buf.Data))
	switch bitDepth {
	case 16:
		for i, s := range buf.Data {
			samples[i] = int16(s)
		}
	case 24:
		for i, s := range buf.Data {
			samples[i] = int16(s >> 8) // Truncate to 16-bit
		}
	case 32:
		for i, s := range buf.Data {
			samples[i] = int16(s >> 16) // Truncate to 16-bit
		}
	default:
		for i, s := range buf.Data {
			samples[i] = int16(s)
		}
	}

	return samples, channels, sampleRate, nil
}
