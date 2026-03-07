package score

import (
	"testing"
)

func TestScoreboardSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	sb := &Scoreboard{}
	entry := Entry{
		Song:       "test_song.mp3",
		SongHash:   "abc123",
		Score:      5000,
		MaxStreak:  25,
		Accuracy:   0.85,
		Difficulty: "medium",
	}

	if err := sb.AddScore(entry); err != nil {
		t.Fatalf("AddScore() error: %v", err)
	}

	// Load it back
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(loaded.Scores))
	}

	if loaded.Scores[0].Score != 5000 {
		t.Errorf("expected score 5000, got %d", loaded.Scores[0].Score)
	}
	if loaded.Scores[0].Player != "default" {
		t.Errorf("expected player 'default', got %s", loaded.Scores[0].Player)
	}
	if loaded.Scores[0].Date == "" {
		t.Error("expected date to be set")
	}
}

func TestScoreboardHighScore(t *testing.T) {
	sb := &Scoreboard{
		Scores: []Entry{
			{Score: 1000, SongHash: "a"},
			{Score: 5000, SongHash: "b"},
			{Score: 3000, SongHash: "a"},
		},
	}

	if sb.HighScore() != 5000 {
		t.Errorf("expected high score 5000, got %d", sb.HighScore())
	}

	if sb.HighScoreForSong("a") != 3000 {
		t.Errorf("expected high score for song 'a' = 3000, got %d", sb.HighScoreForSong("a"))
	}
}

func TestScoreboardTopScores(t *testing.T) {
	sb := &Scoreboard{
		Scores: []Entry{
			{Score: 1000, SongHash: "a"},
			{Score: 5000, SongHash: "a"},
			{Score: 3000, SongHash: "a"},
			{Score: 2000, SongHash: "b"},
		},
	}

	top := sb.TopScoresForSong("a", 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(top))
	}
	if top[0].Score != 5000 {
		t.Errorf("expected first score 5000, got %d", top[0].Score)
	}
	if top[1].Score != 3000 {
		t.Errorf("expected second score 3000, got %d", top[1].Score)
	}
}

func TestScoreboardEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	sb, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(sb.Scores) != 0 {
		t.Errorf("expected 0 scores, got %d", len(sb.Scores))
	}

	if sb.HighScore() != 0 {
		t.Errorf("expected high score 0, got %d", sb.HighScore())
	}
}
