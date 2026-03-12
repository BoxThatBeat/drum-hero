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

// noteHalf represents which half of a cell a note occupies.
type noteHalf int

const (
	noteNone  noteHalf = iota
	noteUpper          // ▀ (upper half-block)
	noteLower          // ▄ (lower half-block)
	noteFull           // █ (full block — two notes in same cell)
)

// Pre-cached styles to avoid per-frame allocation.
type cachedStyles struct {
	laneCell    lipgloss.Style            // empty cell with Width+Align
	sepRendered string                    // pre-rendered separator
	drumActive  map[config.DrumType]lipgloss.Style // bold colored per drum
}

type playModel struct {
	cfg      *config.Config
	engine   *game.Engine
	player   *audio.Player
	songName string
	lanes    []config.DrumType
	styles   cachedStyles
}

func newPlayModel(cfg *config.Config, engine *game.Engine, player *audio.Player, songName string) playModel {
	lanes := config.AllDrumTypes()

	// Pre-cache styles
	cs := cachedStyles{
		laneCell: lipgloss.NewStyle().
			Width(laneWidth).
			Align(lipgloss.Center),
		sepRendered: dimStyle.Render(laneSep),
		drumActive:  make(map[config.DrumType]lipgloss.Style, len(lanes)),
	}
	for _, dt := range lanes {
		vis := DrumVisuals[dt]
		cs.drumActive[dt] = lipgloss.NewStyle().Foreground(vis.Color).Bold(true)
	}

	return playModel{
		cfg:      cfg,
		engine:   engine,
		player:   player,
		songName: songName,
		lanes:    lanes,
		styles:   cs,
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
	b.Grow(width * height) // rough pre-alloc

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
	headerLines := 2  // headers + sep
	hitZoneLines := 3 // sep + keys + sep
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

	for i, dt := range m.lanes {
		vis := DrumVisuals[dt]
		style := lipgloss.NewStyle().
			Foreground(vis.Color).
			Bold(true).
			Width(laneWidth).
			Align(lipgloss.Center)

		if i > 0 {
			headers = append(headers, m.styles.sepRendered)
		}

		headers = append(headers, style.Render(vis.Short))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, headers...)
}

func (m *playModel) renderSeparator(char string) string {
	numLanes := len(m.lanes)
	totalWidth := numLanes*laneWidth + (numLanes - 1) // +separators
	return strings.Repeat(char, totalWidth)
}

// noteSlot represents what's in one half-row slot of the grid.
type noteSlot struct {
	drumType config.DrumType
	consumed bool
	missed   bool
	present  bool
}

func (m *playModel) renderHighway(height int) string {
	if m.engine == nil || m.player == nil {
		return strings.Repeat("\n", height)
	}

	currentMs := m.player.CurrentTimeMs()
	visible := m.engine.VisibleNotes(currentMs, noteWindowAheadMs, noteWindowBehindMs)

	numLanes := len(m.lanes)

	// Double vertical resolution: each terminal row has an upper and lower slot.
	// Total slots = height * 2.
	totalSlots := height * 2

	// Build the grid at double resolution: slots x lanes
	grid := make([][]noteSlot, totalSlots)
	for s := 0; s < totalSlots; s++ {
		grid[s] = make([]noteSlot, numLanes)
	}

	// Map each visible note to a slot
	for _, note := range visible {
		timeDiff := note.Hit.TimeMs - currentMs
		normalized := timeDiff / noteWindowAheadMs
		if normalized < 0 || normalized > 1 {
			continue
		}

		// Slot 0 = top (future), slot totalSlots-1 = bottom (hit zone)
		slot := int((1.0 - normalized) * float64(totalSlots-1))
		if slot < 0 || slot >= totalSlots {
			continue
		}

		// Find the lane for this note
		for col, dt := range m.lanes {
			if note.Hit.Type == dt {
				grid[slot][col] = noteSlot{
					drumType: dt,
					consumed: note.Consumed,
					missed:   note.Missed,
					present:  true,
				}
				break
			}
		}
	}

	// Render the grid: combine pairs of slots into single rows using half-blocks
	var b strings.Builder
	b.Grow(height * (numLanes*(laneWidth+4) + 1))

	for row := 0; row < height; row++ {
		upperIdx := row * 2
		lowerIdx := row*2 + 1

		for col := 0; col < numLanes; col++ {
			if col > 0 {
				b.WriteString(m.styles.sepRendered)
			}

			upper := grid[upperIdx][col]
			lower := grid[lowerIdx][col]

			var cellContent string

			switch {
			case upper.present && lower.present:
				// Both halves have notes — render full block with upper note's color
				cellContent = m.renderNoteChar("█", upper)
			case upper.present:
				cellContent = m.renderNoteChar("▀", upper)
			case lower.present:
				cellContent = m.renderNoteChar("▄", lower)
			default:
				cellContent = " "
			}

			b.WriteString(m.styles.laneCell.Render(cellContent))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderNoteChar renders a half-block character with the appropriate style for a note.
func (m *playModel) renderNoteChar(char string, slot noteSlot) string {
	if slot.consumed {
		return dimStyle.Render(char)
	}
	if slot.missed {
		return hitWrongStyle.Render(char)
	}
	if style, ok := m.styles.drumActive[slot.drumType]; ok {
		return style.Render(char)
	}
	return char
}

func (m *playModel) renderHitZone() string {
	hitResult, hitLane, hitFeedback := m.engine.LastHitFeedback()
	drumToKey := m.cfg.DrumToKey()

	// Top separator (hit zone boundary)
	topSep := m.renderSeparator("═")
	topLine := lipgloss.NewStyle().Foreground(colorBrightWhite).Bold(true).Render(topSep)

	// Key labels with hit/miss flash feedback
	var keyParts []string
	for i, dt := range m.lanes {
		keyLabel := drumToKey[dt]
		style := m.styles.laneCell

		if hitFeedback > 0 && hitLane == dt {
			if hitResult == game.HitCorrect {
				style = style.Foreground(colorBrightGreen).Bold(true)
			} else {
				style = style.Foreground(colorBrightRed).Bold(true)
			}
		} else {
			style = style.Foreground(colorDim)
		}

		if i > 0 {
			keyParts = append(keyParts, m.styles.sepRendered)
		}
		keyParts = append(keyParts, style.Render(keyLabel))
	}
	keyLine := lipgloss.JoinHorizontal(lipgloss.Top, keyParts...)

	// Bottom separator
	botLine := lipgloss.NewStyle().Foreground(colorBrightWhite).Bold(true).Render(topSep)

	return topLine + "\n" + keyLine + "\n" + botLine
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
