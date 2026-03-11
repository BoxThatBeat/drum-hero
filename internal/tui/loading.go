package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type loadingModel struct {
	messages []string
	frame    int
	done     bool
	ready    bool // true when processing succeeded and waiting for user to press enter
	starting bool // true after user pressed enter, waiting for audio to initialize
	errored  bool // true when processing failed
}

func newLoadingModel() loadingModel {
	return loadingModel{
		messages: []string{"Starting..."},
	}
}

func (m *loadingModel) addMessage(msg string) {
	m.messages = append(m.messages, msg)
}

func (m *loadingModel) setDone() {
	m.done = true
}

func (m *loadingModel) setReady() {
	m.done = true
	m.ready = true
}

func (m *loadingModel) setErrored() {
	m.done = true
	m.errored = true
}

func (m *loadingModel) isReady() bool {
	return m.ready && !m.starting
}

func (m *loadingModel) setStarting() {
	m.starting = true
	m.ready = false
}

func (m *loadingModel) tick() {
	m.frame++
}

func (m *loadingModel) view(width, height int) string {
	var b strings.Builder

	b.WriteString("\n\n")
	b.WriteString(menuTitleStyle.Render("  Processing Song"))
	b.WriteString("\n\n")

	// Spinner
	if !m.done {
		spinIdx := (m.frame / 3) % len(spinnerFrames)
		spinner := loadingStyle.Render(spinnerFrames[spinIdx])
		b.WriteString(fmt.Sprintf("  %s Loading...\n\n", spinner))
	} else if m.ready {
		b.WriteString("  Done!\n\n")
	} else {
		b.WriteString("  Error!\n\n")
	}

	// Messages
	start := 0
	if len(m.messages) > 10 {
		start = len(m.messages) - 10
	}
	for _, msg := range m.messages[start:] {
		b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(msg)))
	}

	b.WriteString("\n")
	if m.starting {
		spinIdx := (m.frame / 3) % len(spinnerFrames)
		spinner := loadingStyle.Render(spinnerFrames[spinIdx])
		b.WriteString(fmt.Sprintf("  %s Starting...", spinner))
	} else if m.ready {
		b.WriteString(menuSelectedStyle.Render("  [Enter] Play!"))
		b.WriteString("  ")
		b.WriteString(dimStyle.Render("[q] Back"))
	} else if m.errored {
		b.WriteString(dimStyle.Render("  Returning to menu..."))
	} else {
		b.WriteString(dimStyle.Render("  [q] Cancel"))
	}

	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}
