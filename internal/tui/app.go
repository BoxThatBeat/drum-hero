package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/boxthatbeat/drum-hero/internal/analysis"
	"github.com/boxthatbeat/drum-hero/internal/audio"
	"github.com/boxthatbeat/drum-hero/internal/config"
	"github.com/boxthatbeat/drum-hero/internal/game"
	"github.com/boxthatbeat/drum-hero/internal/score"
)

// tickMsg is sent on every frame tick for animations and game updates.
type tickMsg time.Time

// songLoadedMsg is sent when song processing completes.
type songLoadedMsg struct {
	hash    string
	drumMap *analysis.DrumMap
	err     error
}

// progressCollector safely collects progress messages from a background goroutine.
type progressCollector struct {
	mu       sync.Mutex
	messages []string
}

func (pc *progressCollector) add(msg string) {
	pc.mu.Lock()
	pc.messages = append(pc.messages, msg)
	pc.mu.Unlock()
}

func (pc *progressCollector) drain() []string {
	pc.mu.Lock()
	msgs := pc.messages
	pc.messages = nil
	pc.mu.Unlock()
	return msgs
}

// App is the root Bubble Tea model.
type App struct {
	cfg        *config.Config
	scoreboard *score.Scoreboard
	state      game.State
	width      int
	height     int

	// Sub-models
	menu    menuModel
	loading loadingModel
	play    playModel
	results resultsModel

	// Song being processed/played
	songPath string
	songHash string
	songName string

	// Progress collector for background processing
	progress *progressCollector

	// Game components (created during loading, used during gameplay)
	player *audio.Player
	engine *game.Engine
}

// NewApp creates a new App with the given configuration.
func NewApp(cfg *config.Config, initialSong string) *App {
	sb, _ := score.Load()
	if sb == nil {
		sb = &score.Scoreboard{}
	}

	app := &App{
		cfg:        cfg,
		scoreboard: sb,
		state:      game.StateMenu,
		progress:   &progressCollector{},
	}

	app.menu = newMenuModel(cfg)

	if initialSong != "" {
		app.songPath = initialSong
		app.songName = songNameFromPath(initialSong)
		app.state = game.StateLoading
		app.loading = newLoadingModel()
	}

	return app
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	if a.state == game.StateLoading {
		return tea.Batch(
			a.tickCmd(),
			a.loadSongCmd(a.songPath),
		)
	}
	return a.tickCmd()
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			a.cleanup()
			return a, tea.Quit
		}
	}

	// Dispatch to current state handler
	switch a.state {
	case game.StateMenu:
		return a.updateMenu(msg)
	case game.StateLoading:
		return a.updateLoading(msg)
	case game.StatePlaying:
		return a.updatePlaying(msg)
	case game.StatePaused:
		return a.updatePaused(msg)
	case game.StateResults:
		return a.updateResults(msg)
	}

	return a, nil
}

// View implements tea.Model.
func (a *App) View() tea.View {
	var content string

	switch a.state {
	case game.StateMenu:
		content = a.menu.view(a.width, a.height, a.scoreboard.HighScore())
	case game.StateLoading:
		content = a.loading.view(a.width, a.height)
	case game.StatePlaying:
		content = a.play.view(a.width, a.height, a.scoreboard.HighScore())
	case game.StatePaused:
		content = a.play.viewWithPause(a.width, a.height, a.scoreboard.HighScore())
	case game.StateResults:
		content = a.results.view(a.width, a.height)
	default:
		content = "Unknown state"
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// --- State transition handlers ---

func (a *App) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "escape":
			a.cleanup()
			return a, tea.Quit
		case "enter":
			selected := a.menu.selectedPath()
			if selected != "" {
				a.songPath = selected
				a.songName = a.menu.selectedName()
				a.state = game.StateLoading
				a.loading = newLoadingModel()
				a.progress = &progressCollector{}
				return a, tea.Batch(
					a.tickCmd(),
					a.loadSongCmd(selected),
				)
			}
		case "up", "k":
			a.menu.moveUp()
		case "down", "j":
			a.menu.moveDown()
		}
	case tickMsg:
		return a, a.tickCmd()
	}
	return a, nil
}

func (a *App) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "escape" {
			a.state = game.StateMenu
			return a, nil
		}
	case songLoadedMsg:
		if msg.err != nil {
			a.loading.addMessage(fmt.Sprintf("Error: %v", msg.err))
			a.loading.setDone()
			// Stay on loading screen for 3 seconds, then return to menu
			return a, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return songLoadedMsg{err: fmt.Errorf("returning to menu")}
			})
		}
		if a.loading.isDone() {
			// This is the delayed return-to-menu after an error
			a.state = game.StateMenu
			return a, nil
		}
		a.songHash = msg.hash
		return a, a.startGameplay(msg.drumMap)
	case tickMsg:
		// Drain progress messages from the collector
		for _, msg := range a.progress.drain() {
			a.loading.addMessage(msg)
		}
		a.loading.tick()

		// Check if song is finished
		if a.player != nil && a.player.IsFinished() {
			return a, a.finishGame()
		}

		return a, a.tickCmd()
	}
	return a, nil
}

func (a *App) updatePlaying(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "escape" {
			if a.player != nil {
				a.player.Pause()
			}
			a.state = game.StatePaused
			return a, nil
		}
		// Game input
		if a.engine != nil {
			a.engine.KeyPress(key)
		}

	case tickMsg:
		if a.engine != nil {
			a.engine.Update()
		}

		if a.player != nil && a.player.IsFinished() {
			return a, a.finishGame()
		}

		return a, a.tickCmd()
	}
	return a, nil
}

func (a *App) updatePaused(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "escape", "space":
			if a.player != nil {
				a.player.Resume()
			}
			a.state = game.StatePlaying
			return a, a.tickCmd()
		case "q":
			a.cleanup()
			a.state = game.StateMenu
			return a, nil
		}
	case tickMsg:
		return a, a.tickCmd()
	}
	return a, nil
}

func (a *App) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter", "space", "q", "escape":
			a.state = game.StateMenu
			return a, nil
		}
	case tickMsg:
		return a, a.tickCmd()
	}
	return a, nil
}

// --- Commands ---

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) loadSongCmd(path string) tea.Cmd {
	pc := a.progress
	return func() tea.Msg {
		onProgress := func(msg string) {
			pc.add(msg)
		}

		hash, err := audio.Separate(path, onProgress)
		if err != nil {
			return songLoadedMsg{err: err}
		}

		drumMap, err := analysis.Analyze(hash, onProgress)
		if err != nil {
			return songLoadedMsg{err: err}
		}

		return songLoadedMsg{hash: hash, drumMap: drumMap}
	}
}

func (a *App) startGameplay(drumMap *analysis.DrumMap) tea.Cmd {
	return func() tea.Msg {
		player, err := audio.NewPlayer()
		if err != nil {
			return songLoadedMsg{err: fmt.Errorf("creating audio player: %w", err)}
		}

		if err := player.Load(
			cacheNoDrumsPath(a.songHash),
			cacheDrumsPath(a.songHash),
		); err != nil {
			player.Close()
			return songLoadedMsg{err: fmt.Errorf("loading audio tracks: %w", err)}
		}

		if err := player.Start(); err != nil {
			player.Close()
			return songLoadedMsg{err: fmt.Errorf("starting playback: %w", err)}
		}

		a.player = player
		a.engine = game.NewEngine(a.cfg, drumMap, player)
		a.play = newPlayModel(a.cfg, a.engine, player, a.songName)
		a.state = game.StatePlaying

		return tickMsg(time.Now())
	}
}

func (a *App) finishGame() tea.Cmd {
	return func() tea.Msg {
		if a.engine != nil {
			s := a.engine.Score()
			entry := score.Entry{
				Song:       a.songName,
				SongHash:   a.songHash,
				Score:      s.Points,
				MaxStreak:  s.MaxStreak,
				Accuracy:   s.Accuracy(),
				Difficulty: string(a.cfg.Difficulty.Preset),
			}
			_ = a.scoreboard.AddScore(entry)
			a.results = newResultsModel(s, a.songName, a.scoreboard, a.songHash)
		}

		a.state = game.StateResults

		if a.player != nil {
			a.player.Stop()
		}

		return tickMsg(time.Now())
	}
}

func (a *App) cleanup() {
	if a.player != nil {
		a.player.Close()
		a.player = nil
	}
}

// --- Helper functions ---

func cacheNoDrumsPath(hash string) string {
	return cachePkg.NoDrumsPath(hash)
}

func cacheDrumsPath(hash string) string {
	return cachePkg.DrumsPath(hash)
}

func songNameFromPath(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}
