package game

import (
	"math"
	"sync"

	"github.com/boxthatbeat/drum-hero/internal/analysis"
	"github.com/boxthatbeat/drum-hero/internal/audio"
	"github.com/boxthatbeat/drum-hero/internal/config"
)

// HitResult represents the result of a player's key press.
type HitResult int

const (
	HitCorrect HitResult = iota
	HitWrong
	HitMiss // note passed without being hit
)

// NoteState tracks the state of each drum hit note during gameplay.
type NoteState struct {
	Hit      analysis.DrumHit
	Index    int       // original index in the drum map
	Consumed bool      // whether this note has been matched
	Missed   bool      // whether this note was missed (passed the hit zone)
	Result   HitResult // the result if consumed
}

// Engine is the core game logic engine.
type Engine struct {
	cfg     *config.Config
	drumMap *analysis.DrumMap
	player  *audio.Player
	score   *Score

	// Note tracking
	notes       []NoteState
	notesMu     sync.RWMutex
	nextNoteIdx int // index of the next note to check for misses

	// Key mapping
	keyToDrum map[string]config.DrumType

	// Timing
	thresholdMs float64

	// Hit feedback for TUI
	lastHitResult   HitResult
	lastHitLane     config.DrumType
	lastHitFeedback int // countdown frames for showing feedback

	mu sync.Mutex
}

// NewEngine creates a new game engine.
func NewEngine(cfg *config.Config, drumMap *analysis.DrumMap, player *audio.Player) *Engine {
	notes := make([]NoteState, len(drumMap.Hits))
	for i, hit := range drumMap.Hits {
		notes[i] = NoteState{
			Hit:   hit,
			Index: i,
		}
	}

	return &Engine{
		cfg:         cfg,
		drumMap:     drumMap,
		player:      player,
		score:       NewScore(),
		notes:       notes,
		keyToDrum:   cfg.KeyToDrum(),
		thresholdMs: float64(cfg.ThresholdMs()),
	}
}

// Score returns the current score.
func (e *Engine) Score() *Score {
	return e.score
}

// Notes returns the current note states (for rendering).
func (e *Engine) Notes() []NoteState {
	e.notesMu.RLock()
	defer e.notesMu.RUnlock()
	return e.notes
}

// DrumMap returns the drum map.
func (e *Engine) DrumMap() *analysis.DrumMap {
	return e.drumMap
}

// ThresholdMs returns the timing threshold in milliseconds.
func (e *Engine) ThresholdMs() float64 {
	return e.thresholdMs
}

// LastHitFeedback returns the last hit result, lane, and remaining feedback frames.
func (e *Engine) LastHitFeedback() (HitResult, config.DrumType, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastHitResult, e.lastHitLane, e.lastHitFeedback
}

// KeyPress handles a key press from the player.
// Returns the hit result and matched drum type (if any).
func (e *Engine) KeyPress(key string) (HitResult, config.DrumType) {
	drumType, ok := e.keyToDrum[key]
	if !ok {
		return HitWrong, ""
	}

	currentMs := e.player.CurrentTimeMs()

	e.notesMu.Lock()
	defer e.notesMu.Unlock()

	// Find the closest unconsumed note of this drum type within the threshold
	bestIdx := -1
	bestDist := math.MaxFloat64

	for i := range e.notes {
		n := &e.notes[i]
		if n.Consumed || n.Missed {
			continue
		}
		if n.Hit.Type != drumType {
			continue
		}

		dist := math.Abs(n.Hit.TimeMs - currentMs)
		if dist <= e.thresholdMs && dist < bestDist {
			bestIdx = i
			bestDist = dist
		}
	}

	if bestIdx >= 0 {
		// Correct hit!
		e.notes[bestIdx].Consumed = true
		e.notes[bestIdx].Result = HitCorrect
		e.score.Hit()

		// Unmute drums around this hit — window duration is configurable
		// so the drum sound rings out naturally without affecting hit scoring.
		unmuteMs := float64(e.cfg.Audio.DrumUnmuteMs)
		e.player.UnmuteDrums(e.notes[bestIdx].Hit.TimeMs, 50, unmuteMs)

		e.mu.Lock()
		e.lastHitResult = HitCorrect
		e.lastHitLane = drumType
		e.lastHitFeedback = 8 // frames to show feedback
		e.mu.Unlock()

		return HitCorrect, drumType
	}

	// Wrong timing or wrong type
	e.score.Wrong()

	e.mu.Lock()
	e.lastHitResult = HitWrong
	e.lastHitLane = drumType
	e.lastHitFeedback = 8
	e.mu.Unlock()

	return HitWrong, drumType
}

// Update checks for missed notes and updates feedback counters.
// Should be called every frame/tick.
func (e *Engine) Update() {
	currentMs := e.player.CurrentTimeMs()

	e.notesMu.Lock()
	// Check for notes that have passed the hit zone
	for i := e.nextNoteIdx; i < len(e.notes); i++ {
		n := &e.notes[i]
		if n.Consumed || n.Missed {
			if i == e.nextNoteIdx {
				e.nextNoteIdx++
			}
			continue
		}

		// If the note is past the hit zone (current time > note time + threshold)
		if currentMs > n.Hit.TimeMs+e.thresholdMs {
			n.Missed = true
			n.Result = HitMiss
			e.score.Miss()
			if i == e.nextNoteIdx {
				e.nextNoteIdx++
			}
		} else {
			// Notes are in order, so if this one isn't missed yet, later ones won't be either
			break
		}
	}
	e.notesMu.Unlock()

	// Decay feedback counter
	e.mu.Lock()
	if e.lastHitFeedback > 0 {
		e.lastHitFeedback--
	}
	e.mu.Unlock()

	// Periodically clean up old unmute windows
	e.player.CleanupUnmuteWindows()
}

// VisibleNotes returns notes that should be visible on screen.
// windowBeforeMs: how far ahead to show notes (above the hit zone)
// windowAfterMs: how far behind to keep notes (below the hit zone, for feedback)
func (e *Engine) VisibleNotes(currentMs, windowBeforeMs, windowAfterMs float64) []NoteState {
	e.notesMu.RLock()
	defer e.notesMu.RUnlock()

	startMs := currentMs - windowAfterMs
	endMs := currentMs + windowBeforeMs

	var visible []NoteState
	for _, n := range e.notes {
		if n.Hit.TimeMs >= startMs && n.Hit.TimeMs <= endMs {
			visible = append(visible, n)
		}
		if n.Hit.TimeMs > endMs {
			break // notes are sorted
		}
	}

	return visible
}

// IsFinished returns whether all notes have been consumed or missed.
func (e *Engine) IsFinished() bool {
	e.notesMu.RLock()
	defer e.notesMu.RUnlock()
	return e.nextNoteIdx >= len(e.notes)
}
