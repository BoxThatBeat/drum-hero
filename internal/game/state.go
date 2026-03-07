package game

// State represents the current state of the game.
type State int

const (
	StateMenu    State = iota // Song selection menu
	StateLoading              // Processing song (demucs + analysis)
	StatePlaying              // Gameplay in progress
	StatePaused               // Gameplay paused
	StateResults              // Showing results after song ends
)

// String returns a human-readable name for the state.
func (s State) String() string {
	switch s {
	case StateMenu:
		return "Menu"
	case StateLoading:
		return "Loading"
	case StatePlaying:
		return "Playing"
	case StatePaused:
		return "Paused"
	case StateResults:
		return "Results"
	default:
		return "Unknown"
	}
}
