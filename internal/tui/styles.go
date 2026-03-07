package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/boxthatbeat/drum-hero/internal/config"
)

// ANSI colors (0-15) to respect terminal theme.
var (
	colorRed           color.Color = lipgloss.ANSIColor(1)
	colorGreen         color.Color = lipgloss.ANSIColor(2)
	colorYellow        color.Color = lipgloss.ANSIColor(3)
	colorBlue          color.Color = lipgloss.ANSIColor(4)
	colorMagenta       color.Color = lipgloss.ANSIColor(5)
	colorCyan          color.Color = lipgloss.ANSIColor(6)
	colorWhite         color.Color = lipgloss.ANSIColor(7)
	colorBrightRed     color.Color = lipgloss.ANSIColor(9)
	colorBrightGreen   color.Color = lipgloss.ANSIColor(10)
	colorBrightYellow  color.Color = lipgloss.ANSIColor(11)
	colorBrightBlue    color.Color = lipgloss.ANSIColor(12)
	colorBrightMagenta color.Color = lipgloss.ANSIColor(13)
	colorBrightCyan    color.Color = lipgloss.ANSIColor(14)
	colorBrightWhite   color.Color = lipgloss.ANSIColor(15)
	colorDim           color.Color = lipgloss.ANSIColor(8) // bright black / dim
)

// DrumVisual holds the visual representation of a drum type.
type DrumVisual struct {
	Symbol string
	Color  color.Color
	Label  string
	Short  string // short 2-3 char label
}

// DrumVisuals maps drum types to their visual representation.
var DrumVisuals = map[config.DrumType]DrumVisual{
	config.Kick:     {Symbol: "●", Color: colorRed, Label: "KICK", Short: "KCK"},
	config.Snare:    {Symbol: "◆", Color: colorYellow, Label: "SNARE", Short: "SNR"},
	config.ClosedHH: {Symbol: "▲", Color: colorCyan, Label: "C-HH", Short: "CHH"},
	config.OpenHH:   {Symbol: "△", Color: colorBlue, Label: "O-HH", Short: "OHH"},
	config.HiTom:    {Symbol: "■", Color: colorGreen, Label: "HI-TOM", Short: "HIT"},
	config.MidTom:   {Symbol: "◼", Color: colorMagenta, Label: "MID-TOM", Short: "MDT"},
	config.LowTom:   {Symbol: "▬", Color: colorWhite, Label: "LOW-TOM", Short: "LWT"},
	config.Cymbal:   {Symbol: "★", Color: colorBrightYellow, Label: "CYMBAL", Short: "CYM"},
}

// Styles used throughout the TUI.
var (
	// Title bar style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightWhite).
			Padding(0, 1)

	// Score display style
	scoreStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightGreen)

	// Streak display style
	streakStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightYellow)

	// Multiplier style
	multiplierStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightRed)

	// High score style
	highScoreStyle = lipgloss.NewStyle().
			Foreground(colorBrightMagenta)

	// Lane header style
	laneHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Align(lipgloss.Center)

	// Hit zone style (correct)
	hitCorrectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightGreen)

	// Hit zone style (miss/wrong)
	hitWrongStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightRed)

	// Dim style for inactive elements
	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	// Menu styles
	menuTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightCyan).
			MarginBottom(1)

	menuItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(colorBrightGreen).
				Bold(true).
				PaddingLeft(1)

	// Border style
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDim)

	// Pause overlay style
	pauseStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrightYellow).
			Align(lipgloss.Center).
			Width(30).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorBrightYellow).
			Padding(1, 2)

	// Results header style
	resultsHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorBrightCyan).
				Align(lipgloss.Center).
				MarginBottom(1)

	// Loading spinner style
	loadingStyle = lipgloss.NewStyle().
			Foreground(colorBrightCyan)
)

// laneWidth is the width of each lane column in characters.
const laneWidth = 7
