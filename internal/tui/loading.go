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

func (m *loadingModel) isDone() bool {
	return m.done
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
	} else {
		b.WriteString("  Done!\n\n")
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
	b.WriteString(dimStyle.Render("  [q] Cancel"))

	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}
