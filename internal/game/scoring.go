package game

const (
	// BasePoints is the base score for each correct hit.
	BasePoints = 100

	// Streak thresholds for multiplier increases.
	Streak2x = 10
	Streak4x = 20
	Streak8x = 30
)

// Score tracks the player's score and streak.
type Score struct {
	Points      int
	Streak      int
	MaxStreak   int
	Multiplier  int
	TotalHits   int
	CorrectHits int
	MissedHits  int
	WrongHits   int
}

// NewScore creates a new Score with initial values.
func NewScore() *Score {
	return &Score{
		Multiplier: 1,
	}
}

// Hit records a correct hit.
func (s *Score) Hit() {
	s.Streak++
	s.CorrectHits++
	s.TotalHits++

	// Update multiplier based on streak
	s.updateMultiplier()

	// Add points with multiplier
	s.Points += BasePoints * s.Multiplier

	// Track max streak
	if s.Streak > s.MaxStreak {
		s.MaxStreak = s.Streak
	}
}

// Miss records a missed note (note passed without being hit).
func (s *Score) Miss() {
	s.MissedHits++
	s.TotalHits++
	s.resetStreak()
}

// Wrong records a wrong key press (wrong drum type or bad timing).
func (s *Score) Wrong() {
	s.WrongHits++
	s.resetStreak()
}

// Accuracy returns the hit accuracy as a float in [0, 1].
func (s *Score) Accuracy() float64 {
	if s.TotalHits == 0 {
		return 0
	}
	return float64(s.CorrectHits) / float64(s.TotalHits)
}

// AccuracyPercent returns accuracy as a percentage.
func (s *Score) AccuracyPercent() float64 {
	return s.Accuracy() * 100
}

func (s *Score) updateMultiplier() {
	switch {
	case s.Streak >= Streak8x:
		s.Multiplier = 8
	case s.Streak >= Streak4x:
		s.Multiplier = 4
	case s.Streak >= Streak2x:
		s.Multiplier = 2
	default:
		s.Multiplier = 1
	}
}

func (s *Score) resetStreak() {
	s.Streak = 0
	s.Multiplier = 1
}
