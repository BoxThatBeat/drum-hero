package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/boxthatbeat/drum-hero/internal/audio"
	"github.com/boxthatbeat/drum-hero/internal/config"
)

type songEntry struct {
	name string
	path string
}

type menuModel struct {
	songs       []songEntry
	cursor      int
	songsDir    string
	demucsOk    bool
	demucsError string
	cfg         *config.Config
}

func newMenuModel(cfg *config.Config) menuModel {
	m := menuModel{
		songsDir: cfg.ExpandedSongsDir(),
		cfg:      cfg,
	}
	m.loadSongs()

	// Check demucs availability
	if err := audio.CheckDemucs(); err != nil {
		m.demucsError = err.Error()
	} else {
		m.demucsOk = true
	}

	return m
}

func (m *menuModel) loadSongs() {
	m.songs = nil

	entries, err := os.ReadDir(m.songsDir)
	if err != nil {
		return
	}

	audioExts := map[string]bool{
		".wav": true, ".mp3": true, ".flac": true, ".ogg": true,
		".m4a": true, ".wma": true, ".aac": true,
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if audioExts[ext] {
			m.songs = append(m.songs, songEntry{
				name: e.Name(),
				path: filepath.Join(m.songsDir, e.Name()),
			})
		}
	}
}

func (m *menuModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *menuModel) moveDown() {
	if m.cursor < len(m.songs)-1 {
		m.cursor++
	}
}

func (m *menuModel) selectedPath() string {
	if len(m.songs) == 0 {
		return ""
	}
	return m.songs[m.cursor].path
}

func (m *menuModel) selectedName() string {
	if len(m.songs) == 0 {
		return ""
	}
	return m.songs[m.cursor].name
}

func (m *menuModel) view(width, height, highScore int) string {
	var b strings.Builder

	// Title (ASCII art)
	title := `
 ____                        _   _                
|  _ \ _ __ _   _ _ __ ___  | | | | ___ _ __ ___  
| | | | '__| | | | '_ ' _ \ | |_| |/ _ \ '__/ _ \ 
| |_| | |  | |_| | | | | | ||  _  |  __/ | | (_) |
|____/|_|   \__,_|_| |_| |_||_| |_|\___|_|  \___/ 
`
	b.WriteString(menuTitleStyle.Render(title))
	b.WriteString("\n")

	// Status line
	difficulty := m.cfg.Difficulty.Preset
	thresholdMs := m.cfg.ThresholdMs()
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Difficulty: %s (+/-%dms)",
		strings.ToUpper(string(difficulty)), thresholdMs)))
	b.WriteString("\n")

	if highScore > 0 {
		b.WriteString(highScoreStyle.Render(fmt.Sprintf("  All-Time High Score: %d", highScore)))
		b.WriteString("\n")
	}

	// Demucs status
	if !m.demucsOk {
		b.WriteString("\n")
		b.WriteString(hitWrongStyle.Render(fmt.Sprintf("  WARNING: %s", m.demucsError)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Songs directory: %s", m.songsDir)))
	b.WriteString("\n\n")

	if len(m.songs) == 0 {
		b.WriteString(dimStyle.Render("  No songs found. Add audio files to your songs directory."))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Or run: drum-hero <path-to-audio-file>"))
		b.WriteString("\n")
	} else {
		b.WriteString("  Select a song:\n\n")

		// Show scrollable list (max 15 visible)
		maxVisible := 15
		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.songs) {
			end = len(m.songs)
		}

		for i := start; i < end; i++ {
			song := m.songs[i]
			if i == m.cursor {
				b.WriteString(menuSelectedStyle.Render(fmt.Sprintf("> %s", song.name)))
			} else {
				b.WriteString(menuItemStyle.Render(song.name))
			}
			b.WriteString("\n")
		}

		if len(m.songs) > maxVisible {
			b.WriteString(dimStyle.Render(fmt.Sprintf("\n  (%d songs total)", len(m.songs))))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  [Enter] Play  [j/k] Navigate  [q] Quit"))

	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}
