package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/boxthatbeat/drum-hero/internal/audio"
	"github.com/boxthatbeat/drum-hero/internal/config"
	"github.com/boxthatbeat/drum-hero/internal/game"
)

// How far ahead (in ms) to show notes above the hit zone.
const noteWindowAheadMs = 3000.0

// How far behind (in ms) to keep notes below the hit zone for feedback.
const noteWindowBehindMs = 500.0

// laneSep is the separator character between lanes.
const laneSep = "│"

type playModel struct {
	cfg      *config.Config
	engine   *game.Engine
	player   *audio.Player
	songName string
	lanes    []config.DrumType
}

func newPlayModel(cfg *config.Config, engine *game.Engine, player *audio.Player, songName string) playModel {
	return playModel{
		cfg:      cfg,
		engine:   engine,
		player:   player,
		songName: songName,
		lanes:    config.AllDrumTypes(),
	}
}

func (m *playModel) view(width, height, highScore int) string {
	return m.renderGame(width, height, highScore, false)
}

func (m *playModel) viewWithPause(width, height, highScore int) string {
	return m.renderGame(width, height, highScore, true)
}

func (m *playModel) renderGame(width, height, highScore int, paused bool) string {
	if m.engine == nil {
		return "Loading..."
	}

	s := m.engine.Score()

	var b strings.Builder

	// === HUD (top bar) ===
	hud := m.renderHUD(s, highScore, width)
	b.WriteString(hud)
	b.WriteString("\n")

	// === Lane headers ===
	headers := m.renderLaneHeaders()
	b.WriteString(headers)
	b.WriteString("\n")

	// === Separator ===
	sep := m.renderSeparator("─")
	b.WriteString(dimStyle.Render(sep))
	b.WriteString("\n")

	// === Note highway ===
	hudLines := 3     // HUD + blank
	headerLines := 3  // headers + keys + sep
	hitZoneLines := 3 // sep + indicators + sep
	footerLines := 2  // sep + footer
	highwayHeight := height - hudLines - headerLines - hitZoneLines - footerLines
	if highwayHeight < 5 {
		highwayHeight = 5
	}

	highway := m.renderHighway(highwayHeight)
	b.WriteString(highway)

	// === Hit zone ===
	hitZone := m.renderHitZone()
	b.WriteString(hitZone)

	// === Footer ===
	b.WriteString("\n")
	if paused {
		b.WriteString(dimStyle.Render(" [ESC] Resume  [Q] Quit to menu"))
	} else {
		b.WriteString(dimStyle.Render(" [ESC] Pause"))
	}

	result := b.String()

	// Overlay pause screen if paused
	if paused {
		pauseBox := pauseStyle.Render("PAUSED\n\n[ESC/Space] Resume\n[Q] Quit to menu")
		result = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, pauseBox,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	return result
}

func (m *playModel) renderHUD(s *game.Score, highScore, width int) string {
	song := titleStyle.Render(truncate(m.songName, 30))

	scoreText := fmt.Sprintf("%s %d", scoreStyle.Render("Score:"), s.Points)
	streakText := fmt.Sprintf("%s %d", streakStyle.Render("Streak:"), s.Streak)

	multColor := colorBrightWhite
	if s.Multiplier >= 8 {
		multColor = colorBrightRed
	} else if s.Multiplier >= 4 {
		multColor = colorBrightYellow
	} else if s.Multiplier >= 2 {
		multColor = colorBrightGreen
	}
	multText := lipgloss.NewStyle().Bold(true).Foreground(multColor).Render(
		fmt.Sprintf("%dx", s.Multiplier))

	left := fmt.Sprintf(" %s  %s  %s  %s", song, scoreText, streakText, multText)
	right := highScoreStyle.Render(fmt.Sprintf("High: %d ", highScore))

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m *playModel) renderLaneHeaders() string {
	var headers []string
	var keys []string
	drumToKey := m.cfg.DrumToKey()

	sepStyle := dimStyle

	for i, dt := range m.lanes {
		vis := DrumVisuals[dt]
		style := lipgloss.NewStyle().
			Foreground(vis.Color).
			Bold(true).
			Width(laneWidth).
			Align(lipgloss.Center)

		keyStyle := lipgloss.NewStyle().
			Foreground(colorDim).
			Width(laneWidth).
			Align(lipgloss.Center)

		if i > 0 {
			headers = append(headers, sepStyle.Render(laneSep))
			keys = append(keys, sepStyle.Render(laneSep))
		}

		headers = append(headers, style.Render(vis.Short))
		keys = append(keys, keyStyle.Render(drumToKey[dt]))
	}

	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, headers...)
	keyLine := lipgloss.JoinHorizontal(lipgloss.Top, keys...)

	return headerLine + "\n" + keyLine
}

func (m *playModel) renderSeparator(char string) string {
	numLanes := len(m.lanes)
	totalWidth := numLanes*laneWidth + (numLanes - 1) // +separators
	return strings.Repeat(char, totalWidth)
}

func (m *playModel) renderHighway(height int) string {
	if m.engine == nil || m.player == nil {
		return strings.Repeat("\n", height)
	}

	currentMs := m.player.CurrentTimeMs()
	visible := m.engine.VisibleNotes(currentMs, noteWindowAheadMs, noteWindowBehindMs)

	numLanes := len(m.lanes)
	sepStyle := dimStyle

	// Build the grid: rows x lanes
	grid := make([][]string, height)
	for row := 0; row < height; row++ {
		grid[row] = make([]string, numLanes)
		for col := 0; col < numLanes; col++ {
			grid[row][col] = " "
		}
	}

	// Map each visible note to a row
	for _, note := range visible {
		timeDiff := note.Hit.TimeMs - currentMs
		// Normalize: 0 = hit zone (bottom), 1 = top of screen
		normalized := timeDiff / noteWindowAheadMs
		if normalized < 0 || normalized > 1 {
			continue
		}

		// Row 0 = top (future), row height-1 = bottom (hit zone)
		row := int((1.0 - normalized) * float64(height-1))
		if row < 0 || row >= height {
			continue
		}

		// Find the lane for this note
		for col, dt := range m.lanes {
			if note.Hit.Type == dt {
				vis := DrumVisuals[dt]
				if note.Consumed {
					grid[row][col] = dimStyle.Render(vis.Symbol)
				} else if note.Missed {
					grid[row][col] = hitWrongStyle.Render(vis.Symbol)
				} else {
					style := lipgloss.NewStyle().Foreground(vis.Color).Bold(true)
					grid[row][col] = style.Render(vis.Symbol)
				}
				break
			}
		}
	}

	// Render the grid with lane separators
	var b strings.Builder
	for row := 0; row < height; row++ {
		for col := 0; col < numLanes; col++ {
			if col > 0 {
				b.WriteString(sepStyle.Render(laneSep))
			}
			cell := lipgloss.NewStyle().
				Width(laneWidth).
				Align(lipgloss.Center).
				Render(grid[row][col])
			b.WriteString(cell)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *playModel) renderHitZone() string {
	hitResult, hitLane, hitFeedback := m.engine.LastHitFeedback()
	sepStyle := dimStyle

	// Top separator (hit zone boundary)
	topSep := m.renderSeparator("═")
	topLine := lipgloss.NewStyle().Foreground(colorBrightWhite).Bold(true).Render(topSep)

	// Hit zone indicators
	var hitParts []string
	for i, dt := range m.lanes {
		vis := DrumVisuals[dt]
		indicator := vis.Symbol

		style := lipgloss.NewStyle().
			Width(laneWidth).
			Align(lipgloss.Center)

		if hitFeedback > 0 && hitLane == dt {
			if hitResult == game.HitCorrect {
				style = style.Foreground(vis.Color).Bold(true)
				indicator = "◈"
			} else {
				style = style.Foreground(colorBrightRed).Bold(true)
				indicator = "✗"
			}
		} else {
			style = style.Foreground(colorDim)
		}

		if i > 0 {
			hitParts = append(hitParts, sepStyle.Render(laneSep))
		}
		hitParts = append(hitParts, style.Render(indicator))
	}
	hitLine := lipgloss.JoinHorizontal(lipgloss.Top, hitParts...)

	// Bottom separator
	botLine := lipgloss.NewStyle().Foreground(colorBrightWhite).Bold(true).Render(topSep)

	return topLine + "\n" + hitLine + "\n" + botLine
}

// truncate truncates a string to maxLen, adding "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
