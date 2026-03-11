package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"time"
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

// wavHeader holds parsed WAV file header info.
type wavHeader struct {
	channels   int
	sampleRate int
	bitDepth   int
	dataOffset int64
	dataSize   int
}

// parseWAVHeader reads a WAV file header and locates the data chunk.
func parseWAVHeader(f *os.File) (*wavHeader, error) {
	// Read RIFF header (12 bytes)
	var riffID [4]byte
	var fileSize uint32
	var waveID [4]byte

	if err := binary.Read(f, binary.LittleEndian, &riffID); err != nil {
		return nil, fmt.Errorf("reading RIFF header: %w", err)
	}
	if string(riffID[:]) != "RIFF" {
		return nil, fmt.Errorf("not a RIFF file")
	}
	if err := binary.Read(f, binary.LittleEndian, &fileSize); err != nil {
		return nil, fmt.Errorf("reading file size: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &waveID); err != nil {
		return nil, fmt.Errorf("reading WAVE ID: %w", err)
	}
	if string(waveID[:]) != "WAVE" {
		return nil, fmt.Errorf("not a WAVE file")
	}

	var h wavHeader

	// Scan chunks to find "fmt " and "data"
	foundFmt := false
	for {
		var chunkID [4]byte
		var chunkSize uint32

		if err := binary.Read(f, binary.LittleEndian, &chunkID); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("reading chunk header: %w", err)
		}
		if err := binary.Read(f, binary.LittleEndian, &chunkSize); err != nil {
			return nil, fmt.Errorf("reading chunk size: %w", err)
		}

		id := string(chunkID[:])

		switch id {
		case "fmt ":
			var audioFormat uint16
			var numChannels uint16
			var sampleRate uint32
			var byteRate uint32
			var blockAlign uint16
			var bitsPerSample uint16

			if err := binary.Read(f, binary.LittleEndian, &audioFormat); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}
			if err := binary.Read(f, binary.LittleEndian, &numChannels); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}
			if err := binary.Read(f, binary.LittleEndian, &sampleRate); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}
			if err := binary.Read(f, binary.LittleEndian, &byteRate); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}
			_ = byteRate
			if err := binary.Read(f, binary.LittleEndian, &blockAlign); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}
			_ = blockAlign
			if err := binary.Read(f, binary.LittleEndian, &bitsPerSample); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}

			if audioFormat != 1 {
				return nil, fmt.Errorf("unsupported audio format %d (only PCM/1 supported)", audioFormat)
			}

			h.channels = int(numChannels)
			h.sampleRate = int(sampleRate)
			h.bitDepth = int(bitsPerSample)
			foundFmt = true

			// Skip any extra fmt bytes
			extra := int64(chunkSize) - 16
			if extra > 0 {
				if _, err := f.Seek(extra, io.SeekCurrent); err != nil {
					return nil, err
				}
			}

		case "data":
			if !foundFmt {
				return nil, fmt.Errorf("data chunk before fmt chunk")
			}
			offset, err := f.Seek(0, io.SeekCurrent)
			if err != nil {
				return nil, err
			}
			h.dataOffset = offset
			h.dataSize = int(chunkSize)
			return &h, nil

		default:
			// Skip unknown chunk (pad to even boundary)
			skip := int64(chunkSize)
			if skip%2 != 0 {
				skip++
			}
			if _, err := f.Seek(skip, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skipping chunk %q: %w", id, err)
			}
		}
	}

	return nil, fmt.Errorf("no data chunk found")
}

// DecodeWAV decodes a WAV file into AudioData with float64 samples.
func DecodeWAV(path string) (*AudioData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening wav file: %w", err)
	}
	defer f.Close()

	h, err := parseWAVHeader(f)
	if err != nil {
		return nil, fmt.Errorf("parsing wav header in %s: %w", path, err)
	}

	bytesPerSample := h.bitDepth / 8
	numSamples := h.dataSize / bytesPerSample

	// Read raw data
	rawData := make([]byte, h.dataSize)
	if _, err := io.ReadFull(f, rawData); err != nil {
		return nil, fmt.Errorf("reading wav data: %w", err)
	}

	// Determine the max value for normalization
	var maxVal float64
	switch h.bitDepth {
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

	// Convert to float64
	samples := make([]float64, numSamples)
	switch h.bitDepth {
	case 16:
		for i := 0; i < numSamples; i++ {
			samples[i] = float64(int16(binary.LittleEndian.Uint16(rawData[i*2:]))) / maxVal
		}
	case 24:
		for i := 0; i < numSamples; i++ {
			off := i * 3
			val := int32(rawData[off]) | int32(rawData[off+1])<<8 | int32(rawData[off+2])<<16
			if val >= 1<<23 {
				val -= 1 << 24 // sign extend
			}
			samples[i] = float64(val) / maxVal
		}
	case 32:
		for i := 0; i < numSamples; i++ {
			samples[i] = float64(int32(binary.LittleEndian.Uint32(rawData[i*4:]))) / maxVal
		}
	default:
		return nil, fmt.Errorf("unsupported bit depth: %d", h.bitDepth)
	}

	// Create mono mixdown
	numFrames := numSamples / h.channels
	mono := make([]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		var sum float64
		for ch := 0; ch < h.channels; ch++ {
			sum += samples[i*h.channels+ch]
		}
		mono[i] = sum / float64(h.channels)
	}

	duration := time.Duration(float64(numFrames) / float64(h.sampleRate) * float64(time.Second))

	return &AudioData{
		Samples:    samples,
		Mono:       mono,
		SampleRate: h.sampleRate,
		Channels:   h.channels,
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

	h, err := parseWAVHeader(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("parsing wav header in %s: %w", path, err)
	}

	bytesPerSample := h.bitDepth / 8
	numSamples := h.dataSize / bytesPerSample

	// Read raw data
	rawData := make([]byte, h.dataSize)
	if _, err := io.ReadFull(f, rawData); err != nil {
		return nil, 0, 0, fmt.Errorf("reading wav data: %w", err)
	}

	// Convert to int16
	samples := make([]int16, numSamples)
	switch h.bitDepth {
	case 16:
		for i := 0; i < numSamples; i++ {
			samples[i] = int16(binary.LittleEndian.Uint16(rawData[i*2:]))
		}
	case 24:
		for i := 0; i < numSamples; i++ {
			off := i * 3
			val := int32(rawData[off]) | int32(rawData[off+1])<<8 | int32(rawData[off+2])<<16
			if val >= 1<<23 {
				val -= 1 << 24
			}
			samples[i] = int16(val >> 8)
		}
	case 32:
		for i := 0; i < numSamples; i++ {
			samples[i] = int16(int32(binary.LittleEndian.Uint32(rawData[i*4:])) >> 16)
		}
	default:
		return nil, 0, 0, fmt.Errorf("unsupported bit depth: %d", h.bitDepth)
	}

	return samples, h.channels, h.sampleRate, nil
}
