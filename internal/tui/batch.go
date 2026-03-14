package tui

import (
	"fmt"
	"strings"
	"sync/atomic"

	"charm.land/lipgloss/v2"
	"github.com/boxthatbeat/drum-hero/internal/analysis"
	"github.com/boxthatbeat/drum-hero/internal/cache"
	"github.com/boxthatbeat/drum-hero/internal/config"
)

// songStatus represents the processing status of a single song.
type songStatus int

const (
	songPending songStatus = iota
	songProcessing
	songSkipped
	songDone
	songFailed
)

// batchSongEntry holds a song and its batch processing status.
type batchSongEntry struct {
	name   string
	path   string
	status songStatus
	detail string // short status detail (e.g., "cached", "error: ...")
}

// batchModel is the model for the batch processing screen.
type batchModel struct {
	songs     []batchSongEntry
	messages  []string
	frame     int
	done      bool
	cancelled atomic.Bool
	cfg       config.ClassifierConfig

	// Counters
	processed int
	skipped   int
	failed    int
}

func newBatchModel(songs []songEntry, cfg config.ClassifierConfig) batchModel {
	entries := make([]batchSongEntry, len(songs))
	for i, s := range songs {
		entries[i] = batchSongEntry{name: s.name, path: s.path, status: songPending}
	}
	return batchModel{
		songs: entries,
		cfg:   cfg,
	}
}

func (m *batchModel) addMessage(msg string) {
	m.messages = append(m.messages, msg)
}

func (m *batchModel) tick() {
	m.frame++
}

// checkSongCache checks what's already cached for a song.
// Returns (hasSeparation, hasDrumMap, hash, error).
func checkSongCache(path string, fingerprint string) (bool, bool, string, error) {
	hash, err := cache.HashFile(path)
	if err != nil {
		return false, false, "", err
	}

	hasSep := cache.HasSeparation(hash)

	hasDM := false
	if cache.HasDrumMap(hash) {
		dm, err := loadCachedDrumMap(hash)
		if err == nil && dm.ClassifierFingerprint == fingerprint {
			hasDM = true
		}
	}

	return hasSep, hasDM, hash, nil
}

func (m *batchModel) view(width, height int) string {
	var b strings.Builder

	b.WriteString("\n\n")
	b.WriteString(menuTitleStyle.Render("  Batch Processing"))
	b.WriteString("\n\n")

	total := len(m.songs)
	completed := m.processed + m.skipped + m.failed

	// Progress bar
	if total > 0 {
		pct := float64(completed) / float64(total) * 100
		barWidth := 30
		filled := int(float64(barWidth) * float64(completed) / float64(total))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		b.WriteString(fmt.Sprintf("  %s %.0f%% (%d/%d)\n",
			loadingStyle.Render(bar), pct, completed, total))
		b.WriteString(fmt.Sprintf("  %s  %s  %s\n\n",
			lipgloss.NewStyle().Foreground(colorBrightGreen).Render(fmt.Sprintf("✓ %d done", m.processed)),
			dimStyle.Render(fmt.Sprintf("⊘ %d cached", m.skipped)),
			lipgloss.NewStyle().Foreground(colorBrightRed).Render(fmt.Sprintf("✗ %d failed", m.failed)),
		))
	}

	// Spinner for current song
	if !m.done {
		spinIdx := (m.frame / 3) % len(spinnerFrames)
		spinner := loadingStyle.Render(spinnerFrames[spinIdx])

		// Find current song
		for _, s := range m.songs {
			if s.status == songProcessing {
				b.WriteString(fmt.Sprintf("  %s %s\n\n", spinner, s.name))
				break
			}
		}
	} else {
		if m.cancelled.Load() {
			b.WriteString(hitWrongStyle.Render("  Cancelled."))
		} else {
			b.WriteString(menuSelectedStyle.Render("  Complete!"))
		}
		b.WriteString("\n\n")
	}

	// Recent messages (last 8)
	start := 0
	if len(m.messages) > 8 {
		start = len(m.messages) - 8
	}
	for _, msg := range m.messages[start:] {
		b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(msg)))
	}

	// Song list summary (compact: show last few processed + upcoming)
	b.WriteString("\n")
	showCount := 0
	maxShow := 6
	for i := range m.songs {
		s := &m.songs[i]
		if showCount >= maxShow {
			remaining := 0
			for j := i; j < len(m.songs); j++ {
				if m.songs[j].status == songPending {
					remaining++
				}
			}
			if remaining > 0 {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more\n", remaining)))
			}
			break
		}

		var icon, label string
		switch s.status {
		case songDone:
			icon = lipgloss.NewStyle().Foreground(colorBrightGreen).Render("✓")
			label = s.name
		case songSkipped:
			icon = dimStyle.Render("⊘")
			label = dimStyle.Render(s.name + " (cached)")
		case songFailed:
			icon = lipgloss.NewStyle().Foreground(colorBrightRed).Render("✗")
			label = s.name
			if s.detail != "" {
				label += " — " + s.detail
			}
		case songProcessing:
			spinIdx := (m.frame / 3) % len(spinnerFrames)
			icon = loadingStyle.Render(spinnerFrames[spinIdx])
			label = s.name
		case songPending:
			icon = dimStyle.Render("·")
			label = dimStyle.Render(s.name)
		}

		b.WriteString(fmt.Sprintf("  %s %s\n", icon, label))
		showCount++
	}

	b.WriteString("\n")
	if m.done {
		b.WriteString(dimStyle.Render("  [Enter/q] Back to menu"))
	} else {
		b.WriteString(dimStyle.Render("  [q] Cancel"))
	}

	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}

// loadCachedDrumMap loads a drum map from cache for fingerprint comparison.
func loadCachedDrumMap(hash string) (*analysis.DrumMap, error) {
	return analysis.LoadDrumMap(cache.DrumMapPath(hash))
}
