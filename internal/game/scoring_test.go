package game

import (
	"math"
	"testing"
)

func TestScoreHit(t *testing.T) {
	s := NewScore()

	s.Hit()
	if s.Points != 100 {
		t.Errorf("expected 100 points after 1 hit, got %d", s.Points)
	}
	if s.Streak != 1 {
		t.Errorf("expected streak 1, got %d", s.Streak)
	}
	if s.Multiplier != 1 {
		t.Errorf("expected 1x multiplier, got %d", s.Multiplier)
	}
}

func TestScoreMultiplier(t *testing.T) {
	s := NewScore()

	// Hit 10 times to reach 2x
	for i := 0; i < 10; i++ {
		s.Hit()
	}
	if s.Multiplier != 2 {
		t.Errorf("expected 2x at streak 10, got %dx", s.Multiplier)
	}

	// Hit 10 more to reach 4x
	for i := 0; i < 10; i++ {
		s.Hit()
	}
	if s.Multiplier != 4 {
		t.Errorf("expected 4x at streak 20, got %dx", s.Multiplier)
	}

	// Hit 10 more to reach 8x
	for i := 0; i < 10; i++ {
		s.Hit()
	}
	if s.Multiplier != 8 {
		t.Errorf("expected 8x at streak 30, got %dx", s.Multiplier)
	}
}

func TestScoreStreakReset(t *testing.T) {
	s := NewScore()

	for i := 0; i < 15; i++ {
		s.Hit()
	}
	if s.Streak != 15 {
		t.Errorf("expected streak 15, got %d", s.Streak)
	}
	if s.Multiplier != 2 {
		t.Errorf("expected 2x, got %dx", s.Multiplier)
	}

	s.Miss()
	if s.Streak != 0 {
		t.Errorf("expected streak 0 after miss, got %d", s.Streak)
	}
	if s.Multiplier != 1 {
		t.Errorf("expected 1x after miss, got %dx", s.Multiplier)
	}
	if s.MaxStreak != 15 {
		t.Errorf("expected max streak 15, got %d", s.MaxStreak)
	}
}

func TestScoreWrongResetsStreak(t *testing.T) {
	s := NewScore()
	for i := 0; i < 5; i++ {
		s.Hit()
	}
	s.Wrong()
	if s.Streak != 0 {
		t.Errorf("expected streak 0 after wrong, got %d", s.Streak)
	}
}

func TestScoreAccuracy(t *testing.T) {
	s := NewScore()
	s.Hit()
	s.Hit()
	s.Miss()
	s.Hit()

	expected := 0.75
	if math.Abs(s.Accuracy()-expected) > 0.001 {
		t.Errorf("expected accuracy %f, got %f", expected, s.Accuracy())
	}
}

func TestScoreAccuracyEmpty(t *testing.T) {
	s := NewScore()
	if s.Accuracy() != 0 {
		t.Errorf("expected 0 accuracy with no hits, got %f", s.Accuracy())
	}
}

func TestScorePointsCalculation(t *testing.T) {
	s := NewScore()

	// 10 hits at 1x = 1000 points
	for i := 0; i < 10; i++ {
		s.Hit()
	}
	// First 9 at 1x = 900, 10th at 2x = 200, total = 1100
	if s.Points != 1100 {
		t.Errorf("expected 1100 points, got %d", s.Points)
	}
}
