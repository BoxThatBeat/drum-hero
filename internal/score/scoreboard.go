package score

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Entry represents a single score entry on the scoreboard.
type Entry struct {
	Song       string  `json:"song"`
	SongHash   string  `json:"song_hash"`
	Player     string  `json:"player"`
	Score      int     `json:"score"`
	MaxStreak  int     `json:"max_streak"`
	Accuracy   float64 `json:"accuracy"`
	Difficulty string  `json:"difficulty"`
	Date       string  `json:"date"`
}

// Scoreboard holds all score entries.
type Scoreboard struct {
	Scores []Entry `json:"scores"`
}

// dataDir returns the XDG data directory for drum-hero.
func dataDir() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "drum-hero")
}

// ScoreboardPath returns the path to the scoreboard file.
func ScoreboardPath() string {
	return filepath.Join(dataDir(), "scores.json")
}

// Load loads the scoreboard from disk. Returns an empty scoreboard if the file doesn't exist.
func Load() (*Scoreboard, error) {
	path := ScoreboardPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Scoreboard{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading scoreboard: %w", err)
	}

	var sb Scoreboard
	if err := json.Unmarshal(data, &sb); err != nil {
		return nil, fmt.Errorf("parsing scoreboard: %w", err)
	}

	return &sb, nil
}

// Save writes the scoreboard to disk.
func (sb *Scoreboard) Save() error {
	path := ScoreboardPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	data, err := json.MarshalIndent(sb, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling scoreboard: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// AddScore adds a new score entry and saves the scoreboard.
func (sb *Scoreboard) AddScore(entry Entry) error {
	if entry.Date == "" {
		entry.Date = time.Now().Format(time.RFC3339)
	}
	if entry.Player == "" {
		entry.Player = "default"
	}

	sb.Scores = append(sb.Scores, entry)
	return sb.Save()
}

// HighScore returns the highest score overall.
func (sb *Scoreboard) HighScore() int {
	max := 0
	for _, e := range sb.Scores {
		if e.Score > max {
			max = e.Score
		}
	}
	return max
}

// HighScoreForSong returns the highest score for a specific song hash.
func (sb *Scoreboard) HighScoreForSong(songHash string) int {
	max := 0
	for _, e := range sb.Scores {
		if e.SongHash == songHash && e.Score > max {
			max = e.Score
		}
	}
	return max
}

// TopScoresForSong returns the top N scores for a specific song hash, sorted descending.
func (sb *Scoreboard) TopScoresForSong(songHash string, n int) []Entry {
	var songScores []Entry
	for _, e := range sb.Scores {
		if e.SongHash == songHash {
			songScores = append(songScores, e)
		}
	}

	sort.Slice(songScores, func(i, j int) bool {
		return songScores[i].Score > songScores[j].Score
	})

	if len(songScores) > n {
		songScores = songScores[:n]
	}

	return songScores
}

// TopScores returns the top N scores overall, sorted descending.
func (sb *Scoreboard) TopScores(n int) []Entry {
	sorted := make([]Entry, len(sb.Scores))
	copy(sorted, sb.Scores)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}

	return sorted
}
