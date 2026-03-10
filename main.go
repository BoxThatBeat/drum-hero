package main

import (
	"os"

	"github.com/boxthatbeat/drum-hero/internal/config"
	"github.com/boxthatbeat/drum-hero/internal/tui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	model := tui.NewApp(&cfg, "")

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}
