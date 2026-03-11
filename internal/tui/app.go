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
	"github.com/boxthatbeat/drum-hero/internal/logger"
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

// gameReadyMsg is sent when audio player and game engine are ready.
type gameReadyMsg struct {
	player *audio.Player
	engine *game.Engine
	err    error
}

// returnToMenuMsg is sent after a delay to return from error screen to menu.
// The generation field ensures stale messages from previous load attempts are ignored.
type returnToMenuMsg struct {
	generation int
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
	player  *audio.Player
	engine  *game.Engine
	drumMap *analysis.DrumMap // stored after loading, used when user presses enter

	// Generation counter to invalidate stale returnToMenuMsg from previous loads
	loadGeneration int
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
	// Log non-tick messages
	switch msg.(type) {
	case tickMsg:
		// skip tick spam
	default:
		logger.Log("[Update] state=%s msg=%T", a.state, msg)
	}

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
		case "q", "esc":
			a.cleanup()
			return a, tea.Quit
		case "enter":
			selected := a.menu.selectedPath()
			if selected != "" {
				logger.Log("[Menu] selected song: %s", selected)
				a.songPath = selected
				a.songName = a.menu.selectedName()
				a.state = game.StateLoading
				a.loading = newLoadingModel()
				a.progress = &progressCollector{}
				a.loadGeneration++
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
		key := msg.String()
		logger.Log("[Loading] key=%q ready=%v", key, a.loading.isReady())
		if key == "q" || key == "esc" {
			logger.Log("[Loading] user cancelled, returning to menu")
			a.state = game.StateMenu
			return a, nil
		}
		if key == "enter" && a.loading.isReady() {
			logger.Log("[Loading] user pressed enter, starting gameplay")
			a.loading.setStarting()
			return a, a.startGameplay(a.drumMap)
		}
	case songLoadedMsg:
		if msg.err != nil {
			logger.Log("[Loading] songLoadedMsg ERROR: %v", msg.err)
			a.loading.addMessage(fmt.Sprintf("Error: %v", msg.err))
			a.loading.setErrored()
			gen := a.loadGeneration
			return a, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return returnToMenuMsg{generation: gen}
			})
		}
		logger.Log("[Loading] songLoadedMsg OK: hash=%s hits=%d", msg.hash, len(msg.drumMap.Hits))
		a.songHash = msg.hash
		a.drumMap = msg.drumMap
		a.loading.setReady()
		return a, a.tickCmd()
	case gameReadyMsg:
		if msg.err != nil {
			logger.Log("[Loading] gameReadyMsg ERROR: %v", msg.err)
			a.loading.addMessage(fmt.Sprintf("Error: %v", msg.err))
			a.loading.setErrored()
			gen := a.loadGeneration
			return a, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return returnToMenuMsg{generation: gen}
			})
		}
		logger.Log("[Loading] gameReadyMsg OK, transitioning to StatePlaying")
		a.player = msg.player
		a.engine = msg.engine
		a.play = newPlayModel(a.cfg, a.engine, a.player, a.songName)
		a.state = game.StatePlaying
		return a, a.tickCmd()
	case returnToMenuMsg:
		if msg.generation != a.loadGeneration {
			logger.Log("[Loading] ignoring stale returnToMenuMsg (gen=%d, current=%d)", msg.generation, a.loadGeneration)
			return a, nil
		}
		logger.Log("[Loading] returnToMenuMsg gen=%d, going to menu", msg.generation)
		a.state = game.StateMenu
		return a, nil
	case tickMsg:
		// Drain progress messages from the collector
		for _, msg := range a.progress.drain() {
			a.loading.addMessage(msg)
		}
		a.loading.tick()
		return a, a.tickCmd()
	}
	return a, nil
}

func (a *App) updatePlaying(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "esc" {
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
		case "esc", "space":
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
		case "enter", "space", "q", "esc":
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
		logger.Log("[loadSongCmd] starting: path=%s", path)
		onProgress := func(msg string) {
			logger.Log("[loadSongCmd] progress: %s", msg)
			pc.add(msg)
		}

		hash, err := audio.Separate(path, onProgress)
		if err != nil {
			logger.Log("[loadSongCmd] Separate failed: %v", err)
			return songLoadedMsg{err: err}
		}
		logger.Log("[loadSongCmd] Separate OK: hash=%s", hash)

		drumMap, err := analysis.Analyze(hash, onProgress)
		if err != nil {
			logger.Log("[loadSongCmd] Analyze failed: %v", err)
			return songLoadedMsg{err: err}
		}
		logger.Log("[loadSongCmd] Analyze OK: %d hits", len(drumMap.Hits))

		return songLoadedMsg{hash: hash, drumMap: drumMap}
	}
}

func (a *App) startGameplay(drumMap *analysis.DrumMap) tea.Cmd {
	cfg := a.cfg
	songHash := a.songHash
	noDrumsPath := cacheNoDrumsPath(songHash)
	drumsPath := cacheDrumsPath(songHash)
	logger.Log("[startGameplay] noDrumsPath=%s drumsPath=%s", noDrumsPath, drumsPath)
	return func() tea.Msg {
		logger.Log("[startGameplay] creating audio player...")
		player, err := audio.NewPlayer()
		if err != nil {
			logger.Log("[startGameplay] NewPlayer failed: %v", err)
			return gameReadyMsg{err: fmt.Errorf("creating audio player: %w", err)}
		}

		logger.Log("[startGameplay] loading audio tracks...")
		if err := player.Load(noDrumsPath, drumsPath); err != nil {
			logger.Log("[startGameplay] Load failed: %v", err)
			player.Close()
			return gameReadyMsg{err: fmt.Errorf("loading audio tracks: %w", err)}
		}

		logger.Log("[startGameplay] starting playback...")
		if err := player.Start(); err != nil {
			logger.Log("[startGameplay] Start failed: %v", err)
			player.Close()
			return gameReadyMsg{err: fmt.Errorf("starting playback: %w", err)}
		}

		logger.Log("[startGameplay] all OK, creating engine")
		engine := game.NewEngine(cfg, drumMap, player)
		return gameReadyMsg{player: player, engine: engine}
	}
}

func (a *App) finishGame() tea.Cmd {
	if a.engine == nil {
		return nil
	}
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
	a.state = game.StateResults

	if a.player != nil {
		a.player.Stop()
	}

	return a.tickCmd()
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
