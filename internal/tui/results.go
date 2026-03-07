package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/boxthatbeat/drum-hero/internal/game"
	"github.com/boxthatbeat/drum-hero/internal/score"
)

type resultsModel struct {
	score      *game.Score
	songName   string
	scoreboard *score.Scoreboard
	songHash   string
	topScores  []score.Entry
}

func newResultsModel(s *game.Score, songName string, sb *score.Scoreboard, songHash string) resultsModel {
	return resultsModel{
		score:      s,
		songName:   songName,
		scoreboard: sb,
		songHash:   songHash,
		topScores:  sb.TopScoresForSong(songHash, 10),
	}
}

func (m *resultsModel) view(width, height int) string {
	var b strings.Builder

	// Title
	b.WriteString("\n")
	b.WriteString(resultsHeaderStyle.Render("SONG COMPLETE"))
	b.WriteString("\n\n")

	// Song name
	b.WriteString(titleStyle.Render(fmt.Sprintf("  %s", m.songName)))
	b.WriteString("\n\n")

	// Score summary box
	summary := m.renderScoreSummary()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBrightCyan).
		Padding(1, 3).
		Width(40)

	b.WriteString("  ")
	b.WriteString(boxStyle.Render(summary))
	b.WriteString("\n\n")

	// Scoreboard
	if len(m.topScores) > 0 {
		b.WriteString(resultsHeaderStyle.Render("  TOP SCORES"))
		b.WriteString("\n\n")

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorBrightWhite)
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			headerStyle.Width(6).Render("Rank"),
			headerStyle.Width(10).Render("Score"),
			headerStyle.Width(10).Render("Streak"),
			headerStyle.Width(10).Render("Accuracy"),
		))

		for i, entry := range m.topScores {
			rank := fmt.Sprintf("#%d", i+1)
			style := dimStyle
			if i == 0 {
				style = lipgloss.NewStyle().Foreground(colorBrightYellow).Bold(true)
			} else if entry.Score == m.score.Points {
				style = lipgloss.NewStyle().Foreground(colorBrightGreen)
			}

			b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
				style.Width(6).Render(rank),
				style.Width(10).Render(fmt.Sprintf("%d", entry.Score)),
				style.Width(10).Render(fmt.Sprintf("%d", entry.MaxStreak)),
				style.Width(10).Render(fmt.Sprintf("%.1f%%", entry.Accuracy*100)),
			))
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  [enter] Continue  [q] Quit"))

	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}

func (m *resultsModel) renderScoreSummary() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Width(15).Foreground(colorDim)
	valStyle := lipgloss.NewStyle().Bold(true)

	b.WriteString(fmt.Sprintf("%s %s\n",
		labelStyle.Render("Final Score:"),
		valStyle.Foreground(colorBrightGreen).Render(fmt.Sprintf("%d", m.score.Points)),
	))
	b.WriteString(fmt.Sprintf("%s %s\n",
		labelStyle.Render("Max Streak:"),
		valStyle.Foreground(colorBrightYellow).Render(fmt.Sprintf("%d", m.score.MaxStreak)),
	))
	b.WriteString(fmt.Sprintf("%s %s\n",
		labelStyle.Render("Accuracy:"),
		valStyle.Foreground(colorBrightCyan).Render(fmt.Sprintf("%.1f%%", m.score.AccuracyPercent())),
	))
	b.WriteString(fmt.Sprintf("%s %s / %s / %s",
		labelStyle.Render("Hits/Miss/Wrong:"),
		valStyle.Foreground(colorBrightGreen).Render(fmt.Sprintf("%d", m.score.CorrectHits)),
		valStyle.Foreground(colorBrightRed).Render(fmt.Sprintf("%d", m.score.MissedHits)),
		valStyle.Foreground(colorYellow).Render(fmt.Sprintf("%d", m.score.WrongHits)),
	))

	return b.String()
}
