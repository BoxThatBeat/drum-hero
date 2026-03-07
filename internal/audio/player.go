package audio

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"
)

const (
	// playbackSampleRate is the output sample rate.
	playbackSampleRate = 44100
	// playbackChannels is the number of output channels.
	playbackChannels = 2
	// playbackFormat is the output sample format.
	playbackBitDepth = 16
)

// Player handles real-time audio playback with two-track mixing.
type Player struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	// Audio data (loaded before playback starts)
	noDrums []int16 // interleaved stereo PCM
	drums   []int16 // interleaved stereo PCM

	// Playback state (accessed from audio callback thread)
	position atomic.Int64 // current sample position (interleaved index)
	playing  atomic.Bool  // whether playback is active
	paused   atomic.Bool  // whether playback is paused

	// Drum unmuting: a list of time windows where drums should be audible.
	// Uses a mutex because it's written from the game thread and read from audio thread.
	unmuteWindows []unmuteWindow
	unmuteMu      sync.RWMutex

	// Timing
	startTime time.Time
	pauseTime time.Time
	pauseDur  time.Duration

	// Channels for state
	doneCh chan struct{}
}

// unmuteWindow represents a time range where the drum track should be audible.
type unmuteWindow struct {
	startSample int64
	endSample   int64
}

// NewPlayer creates a new audio player.
func NewPlayer() (*Player, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("initializing audio context: %w", err)
	}

	return &Player{
		ctx:    ctx,
		doneCh: make(chan struct{}),
	}, nil
}

// Load loads the two audio tracks for playback.
// Both tracks must have the same sample rate and channel count.
func (p *Player) Load(noDrumsPath, drumsPath string) error {
	noDrums, nCh, nSR, err := LoadRawSamples(noDrumsPath)
	if err != nil {
		return fmt.Errorf("loading no_drums track: %w", err)
	}

	drums, dCh, dSR, err := LoadRawSamples(drumsPath)
	if err != nil {
		return fmt.Errorf("loading drums track: %w", err)
	}

	if nSR != dSR {
		return fmt.Errorf("sample rate mismatch: no_drums=%d, drums=%d", nSR, dSR)
	}
	if nCh != dCh {
		return fmt.Errorf("channel count mismatch: no_drums=%d, drums=%d", nCh, dCh)
	}

	p.noDrums = noDrums
	p.drums = drums

	return nil
}

// Start begins playback.
func (p *Player) Start() error {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = playbackChannels
	deviceConfig.SampleRate = playbackSampleRate
	deviceConfig.PeriodSizeInMilliseconds = 10 // Low latency
	deviceConfig.Periods = 2

	callbacks := malgo.DeviceCallbacks{
		Data: p.audioCallback,
	}

	device, err := malgo.InitDevice(p.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("initializing audio device: %w", err)
	}
	p.device = device

	p.position.Store(0)
	p.playing.Store(true)
	p.paused.Store(false)
	p.startTime = time.Now()
	p.pauseDur = 0

	if err := device.Start(); err != nil {
		return fmt.Errorf("starting audio device: %w", err)
	}

	return nil
}

// Stop stops playback and cleans up.
func (p *Player) Stop() {
	p.playing.Store(false)
	if p.device != nil {
		p.device.Stop()
		p.device.Uninit()
		p.device = nil
	}
}

// Close cleans up all resources.
func (p *Player) Close() {
	p.Stop()
	if p.ctx != nil {
		_ = p.ctx.Uninit()
		p.ctx.Free()
		p.ctx = nil
	}
}

// Pause pauses playback.
func (p *Player) Pause() {
	if !p.paused.Load() {
		p.paused.Store(true)
		p.pauseTime = time.Now()
	}
}

// Resume resumes playback.
func (p *Player) Resume() {
	if p.paused.Load() {
		p.pauseDur += time.Since(p.pauseTime)
		p.paused.Store(false)
	}
}

// IsPaused returns whether playback is paused.
func (p *Player) IsPaused() bool {
	return p.paused.Load()
}

// IsPlaying returns whether playback is active (not finished).
func (p *Player) IsPlaying() bool {
	return p.playing.Load()
}

// IsFinished returns whether the song has finished playing.
func (p *Player) IsFinished() bool {
	pos := p.position.Load()
	return pos >= int64(len(p.noDrums))
}

// CurrentTime returns the current playback position as a time.Duration.
func (p *Player) CurrentTime() time.Duration {
	pos := p.position.Load()
	frames := pos / playbackChannels
	return time.Duration(float64(frames) / float64(playbackSampleRate) * float64(time.Second))
}

// CurrentTimeMs returns the current playback position in milliseconds.
func (p *Player) CurrentTimeMs() float64 {
	return p.CurrentTime().Seconds() * 1000.0
}

// Duration returns the total duration of the loaded audio.
func (p *Player) Duration() time.Duration {
	if len(p.noDrums) == 0 {
		return 0
	}
	frames := int64(len(p.noDrums)) / playbackChannels
	return time.Duration(float64(frames) / float64(playbackSampleRate) * float64(time.Second))
}

// UnmuteDrums signals that drums should be audible for a time window around a hit.
// beforeMs: how many ms before the hit to start unmuting
// afterMs: how many ms after the hit to keep unmuting
func (p *Player) UnmuteDrums(hitTimeMs float64, beforeMs, afterMs float64) {
	startMs := hitTimeMs - beforeMs
	endMs := hitTimeMs + afterMs
	if startMs < 0 {
		startMs = 0
	}

	startSample := int64(startMs / 1000.0 * float64(playbackSampleRate) * playbackChannels)
	endSample := int64(endMs / 1000.0 * float64(playbackSampleRate) * playbackChannels)

	p.unmuteMu.Lock()
	p.unmuteWindows = append(p.unmuteWindows, unmuteWindow{
		startSample: startSample,
		endSample:   endSample,
	})
	p.unmuteMu.Unlock()
}

// Done returns a channel that's closed when playback finishes.
func (p *Player) Done() <-chan struct{} {
	return p.doneCh
}

// audioCallback is called by the audio thread to fill the output buffer.
func (p *Player) audioCallback(pOutput, pInput []byte, frameCount uint32) {
	if p.paused.Load() {
		// Fill with silence when paused
		for i := range pOutput {
			pOutput[i] = 0
		}
		return
	}

	if !p.playing.Load() {
		for i := range pOutput {
			pOutput[i] = 0
		}
		return
	}

	pos := p.position.Load()
	samplesNeeded := int64(frameCount) * playbackChannels
	totalSamples := int64(len(p.noDrums))

	for i := int64(0); i < samplesNeeded; i++ {
		sampleIdx := pos + i
		if sampleIdx >= totalSamples {
			// Past the end — write silence
			byteIdx := i * 2
			if byteIdx+1 < int64(len(pOutput)) {
				pOutput[byteIdx] = 0
				pOutput[byteIdx+1] = 0
			}
			continue
		}

		// Always play the non-drums track
		mix := int32(p.noDrums[sampleIdx])

		// Add drums if we're in an unmute window
		if p.isDrumUnmuted(sampleIdx) && sampleIdx < int64(len(p.drums)) {
			mix += int32(p.drums[sampleIdx])
		}

		// Clip to int16 range
		if mix > 32767 {
			mix = 32767
		}
		if mix < -32768 {
			mix = -32768
		}

		// Write to output buffer (little-endian int16)
		byteIdx := i * 2
		if byteIdx+1 < int64(len(pOutput)) {
			binary.LittleEndian.PutUint16(pOutput[byteIdx:byteIdx+2], uint16(int16(mix)))
		}
	}

	newPos := pos + samplesNeeded
	p.position.Store(newPos)

	// Check if we've reached the end
	if newPos >= totalSamples {
		p.playing.Store(false)
		select {
		case <-p.doneCh:
		default:
			close(p.doneCh)
		}
	}
}

// isDrumUnmuted checks if the given sample position is within any unmute window.
func (p *Player) isDrumUnmuted(sampleIdx int64) bool {
	p.unmuteMu.RLock()
	defer p.unmuteMu.RUnlock()

	for _, w := range p.unmuteWindows {
		if sampleIdx >= w.startSample && sampleIdx < w.endSample {
			return true
		}
	}
	return false
}

// CleanupUnmuteWindows removes expired unmute windows (called periodically from game thread).
func (p *Player) CleanupUnmuteWindows() {
	pos := p.position.Load()
	p.unmuteMu.Lock()
	defer p.unmuteMu.Unlock()

	// Remove windows that are fully in the past
	n := 0
	for _, w := range p.unmuteWindows {
		if w.endSample > pos {
			p.unmuteWindows[n] = w
			n++
		}
	}
	p.unmuteWindows = p.unmuteWindows[:n]
}
