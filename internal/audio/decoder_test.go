package audio

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeTestWAV creates a minimal 16-bit stereo WAV file with a sine wave.
func writeTestWAV(t *testing.T, path string, sampleRate, numFrames, channels int) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	bitsPerSample := 16
	dataSize := numFrames * channels * (bitsPerSample / 8)
	fileSize := 36 + dataSize

	// RIFF header
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(fileSize))
	f.Write([]byte("WAVE"))

	// fmt chunk
	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))         // chunk size
	binary.Write(f, binary.LittleEndian, uint16(1))          // PCM format
	binary.Write(f, binary.LittleEndian, uint16(channels))   // channels
	binary.Write(f, binary.LittleEndian, uint32(sampleRate)) // sample rate
	bytesPerSec := sampleRate * channels * (bitsPerSample / 8)
	binary.Write(f, binary.LittleEndian, uint32(bytesPerSec)) // byte rate
	blockAlign := channels * (bitsPerSample / 8)
	binary.Write(f, binary.LittleEndian, uint16(blockAlign))    // block align
	binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)) // bits per sample

	// data chunk
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataSize))

	// Write sine wave samples
	freq := 440.0
	for i := 0; i < numFrames; i++ {
		sample := math.Sin(2.0 * math.Pi * freq * float64(i) / float64(sampleRate))
		val := int16(sample * math.MaxInt16 * 0.5) // 50% volume
		for ch := 0; ch < channels; ch++ {
			binary.Write(f, binary.LittleEndian, val)
		}
	}
}

func TestDecodeWAV(t *testing.T) {
	tmpDir := t.TempDir()
	wavPath := filepath.Join(tmpDir, "test.wav")

	sampleRate := 44100
	numFrames := 44100 // 1 second
	channels := 2

	writeTestWAV(t, wavPath, sampleRate, numFrames, channels)

	data, err := DecodeWAV(wavPath)
	if err != nil {
		t.Fatalf("DecodeWAV() error: %v", err)
	}

	if data.SampleRate != sampleRate {
		t.Errorf("expected sample rate %d, got %d", sampleRate, data.SampleRate)
	}
	if data.Channels != channels {
		t.Errorf("expected %d channels, got %d", channels, data.Channels)
	}
	if data.NumFrames() != numFrames {
		t.Errorf("expected %d frames, got %d", numFrames, data.NumFrames())
	}
	if len(data.Samples) != numFrames*channels {
		t.Errorf("expected %d samples, got %d", numFrames*channels, len(data.Samples))
	}
	if len(data.Mono) != numFrames {
		t.Errorf("expected %d mono samples, got %d", numFrames, len(data.Mono))
	}

	// Verify normalization: samples should be in [-1.0, 1.0]
	for i, s := range data.Samples {
		if s < -1.0 || s > 1.0 {
			t.Errorf("sample %d out of range: %f", i, s)
			break
		}
	}

	// Duration should be approximately 1 second
	if data.Duration.Seconds() < 0.9 || data.Duration.Seconds() > 1.1 {
		t.Errorf("expected ~1s duration, got %v", data.Duration)
	}
}

func TestFrameAtTime(t *testing.T) {
	data := &AudioData{
		Samples:    make([]float64, 88200), // 1 second stereo
		Mono:       make([]float64, 44100),
		SampleRate: 44100,
		Channels:   2,
	}

	// At 0.5s, frame should be ~22050
	frame := data.FrameAtTime(500_000_000) // 500ms in nanoseconds
	if frame < 22049 || frame > 22051 {
		t.Errorf("expected frame ~22050 at 0.5s, got %d", frame)
	}

	// At 0, frame should be 0
	frame = data.FrameAtTime(0)
	if frame != 0 {
		t.Errorf("expected frame 0 at t=0, got %d", frame)
	}
}

func TestTimeAtFrame(t *testing.T) {
	data := &AudioData{
		SampleRate: 44100,
	}

	dur := data.TimeAtFrame(44100)
	if dur.Seconds() < 0.99 || dur.Seconds() > 1.01 {
		t.Errorf("expected ~1s at frame 44100, got %v", dur)
	}
}

func TestLoadRawSamples(t *testing.T) {
	tmpDir := t.TempDir()
	wavPath := filepath.Join(tmpDir, "test.wav")

	writeTestWAV(t, wavPath, 44100, 44100, 2)

	samples, channels, sampleRate, err := LoadRawSamples(wavPath)
	if err != nil {
		t.Fatalf("LoadRawSamples() error: %v", err)
	}

	if sampleRate != 44100 {
		t.Errorf("expected 44100, got %d", sampleRate)
	}
	if channels != 2 {
		t.Errorf("expected 2 channels, got %d", channels)
	}
	if len(samples) != 44100*2 {
		t.Errorf("expected %d samples, got %d", 44100*2, len(samples))
	}
}
