# Drum Hero — Project Plan

## Overview

A terminal-based rhythm game inspired by Guitar Hero, focused on drumming. The player provides any audio file, the app separates drums using `demucs`, analyzes drum hits via onset detection + spectral classification, and presents a vertical falling-notes TUI where the player must hit the correct keys in time with the music.

---

## Technology Stack

| Component          | Choice                                          | Rationale                                              |
| ------------------ | ----------------------------------------------- | ------------------------------------------------------ |
| Language           | **Go**                                          | Fast, single binary, excellent TUI ecosystem           |
| TUI Framework      | **Bubble Tea v2** (`charm.land/bubbletea/v2`)   | Elm architecture, progressive keyboard support         |
| TUI Styling        | **Lip Gloss v2** (`charm.land/lipgloss/v2`)     | Canvas/Layer compositing, gradient support             |
| Audio I/O          | **malgo** (`github.com/gen2brain/malgo`)         | miniaudio bindings, low-latency, cross-platform        |
| WAV Decoding       | **go-audio/wav** (`github.com/go-audio/wav`)     | Pure Go, handles all PCM formats                       |
| Drum Separation    | **demucs CLI** (shelled out)                     | `htdemucs_ft` model, best quality                      |
| Drum Classification| **Onset detection + spectral frequency heuristic** | No ML dependency, works great on isolated drums      |
| Config             | **TOML** (`github.com/BurntSushi/toml`)          | Human-friendly config format                           |
| Scoreboard         | **JSON** in `~/.local/share/drum-hero/scores.json`| Simple, readable                                     |

---

## Architecture

```
cmd/drum-hero/main.go          -- Entry point, CLI arg parsing
internal/
├── config/
│   └── config.go               -- XDG config loading, key mappings, difficulty presets
├── audio/
│   ├── demucs.go               -- Shell out to demucs, manage separation
│   ├── decoder.go              -- WAV file decoding (go-audio/wav)
│   ├── mixer.go                -- Real-time two-track mixer (non-drums + drums)
│   └── player.go               -- malgo device management, audio playback engine
├── analysis/
│   ├── onset.go                -- Onset detection (spectral flux)
│   ├── classifier.go           -- Drum hit classification (FFT frequency bands)
│   └── drummap.go              -- DrumMap type: timestamped list of classified hits
├── game/
│   ├── engine.go               -- Game loop: timing, hit detection, scoring, streak
│   ├── scoring.go              -- Score calculation, multiplier logic
│   └── state.go                -- Game state machine (menu, loading, playing, paused, results)
├── tui/
│   ├── app.go                  -- Bubble Tea program setup, root model
│   ├── menu.go                 -- Song selection menu model
│   ├── gameplay.go             -- Gameplay model (falling notes, lanes, hit zone)
│   ├── results.go              -- Results screen model (score, scoreboard)
│   ├── loading.go              -- Loading/processing screen model
│   ├── pause.go                -- Pause overlay model
│   └── styles.go               -- Lip Gloss style definitions, colors, symbols
├── score/
│   └── scoreboard.go           -- Scoreboard file I/O (JSON), high score tracking
└── cache/
    └── cache.go                -- XDG cache management, hash-based lookup
```

---

## Detailed Component Design

### 1. Config (`~/.config/drum-hero/config.toml`)

```toml
[keys]
kick = "j"
snare = "f"
closed-hihat = "k"
open-hihat = "l"
hi-tom = "d"
mid-tom = "s"
low-tom = "a"
cymbal = ";"

[difficulty]
preset = "medium"  # easy | medium | hard | expert | custom
custom_threshold_ms = 80  # only used if preset = "custom"

[general]
songs_dir = "~/Music/drum-hero"  # directory to browse for songs
```

**Difficulty presets:**

| Preset | Timing Window |
| ------ | ------------- |
| Easy   | +/-150ms      |
| Medium | +/-100ms      |
| Hard   | +/-60ms       |
| Expert | +/-30ms       |

### 2. Audio Pipeline

#### Separation phase (one-time per song, cached)

1. Hash the input audio file (SHA256)
2. Check `~/.cache/drum-hero/{hash}/` for cached results
3. If not cached, run: `demucs -n htdemucs_ft --two-stems=drums "<input_file>"`
4. This produces `drums.wav` and `no_drums.wav`
5. Store in cache directory along with metadata JSON

#### Analysis phase (one-time per song, cached alongside)

1. Load `drums.wav` into memory
2. Run spectral flux onset detection to find drum hit timestamps
3. For each onset, take a short window (~50ms), compute FFT
4. Classify based on frequency band energy distribution:
   - **Kick**: dominant energy <200Hz
   - **Snare**: energy peak 200Hz-2kHz with broadband noise component
   - **Closed hi-hat**: high energy >5kHz, short decay
   - **Open hi-hat**: high energy >5kHz, longer decay
   - **Hi-tom**: energy peak ~300-600Hz
   - **Mid-tom**: energy peak ~150-350Hz
   - **Low-tom**: energy peak ~80-200Hz
   - **Cymbal**: broad energy >3kHz, long sustain
5. Save as `drummap.json` in cache: `[{time_ms: 1250, type: "snare"}, ...]`

#### Playback phase (real-time)

1. Initialize one malgo playback device
2. In the data callback, mix two streams:
   - `no_drums.wav` -- always playing
   - `drums.wav` -- volume set to 0 by default
3. When the game engine signals a correct hit, set the drum track volume to 1.0 for a short window around that hit (~100ms before, ~200ms after the hit)
4. Use atomic variables or a lock-free ring buffer to communicate between game thread and audio callback

### 3. Game Engine

#### Timing system

- Use `time.Now()` with monotonic clock for precise timing
- Track current song position relative to audio playback start
- All drum hit times are relative to song start

#### Hit detection

- When player presses a key, record timestamp and drum type
- Compare against upcoming drum hits within the difficulty window
- A hit is "correct" if: `|player_time - drum_hit_time| <= threshold_ms` AND `player_drum_type == drum_hit_type`
- Each drum hit can only be matched once (consumed on match)

#### Scoring

- Base: **100 points** per correct hit
- Multiplier: starts at **1x**, doubles at streak milestones:
  - 10 streak -> 2x
  - 20 streak -> 4x
  - 30 streak -> 8x (max)
- Streak resets to 0 on miss (wrong key, wrong timing, or missed note entirely)
- Missed note = note passes through hit zone without being hit

#### Pause

- `Esc` to pause/unpause
- Pausing freezes audio playback, note scrolling, and timing
- Overlay shows "PAUSED" with resume/quit options

### 4. TUI Layout

```
+------------------------------------------------------------+
|  Song: "Through the Fire and Flames"     High Score: 45200 |
|  Score: 12400  |  Streak: 23  |  Multiplier: 4x           |
+------+------+------+------+------+------+------+----------+
| LOW  | MID  | HI   |SNARE | C-HH | O-HH | KICK | CYMBAL  |
| TOM  | TOM  | TOM  |      |      |      |      |         |
|  a   |  s   |  d   |  f   |  k   |  l   |  j   |  ;      |
|      |      |      |      |      |      |      |         |
|      |      |  V   |      |  V   |      |      |         |
|      |      |      |      |      |      |      |         |
|      |  V   |      |  V   |      |      |  V   |         |
|      |      |      |      |      |      |      |         |
|      |      |      |      |  V   |      |      |    V    |
|      |      |  V   |      |      |      |      |         |
|      |      |      |      |      |      |  V   |         |
|      |      |      |  V   |      |      |      |         |
+======+======+======+======+======+======+======+==========+ <- Hit zone
|  <>  |  <>  |  <>  |  <>  |  <>  |  <>  |  <>  |  <>     |
+======+======+======+======+======+======+======+==========+
| [ESC] Pause                                                |
+------------------------------------------------------------+
```

#### Visual elements per drum type

| Drum Type    | Symbol | Terminal Color  |
| ------------ | ------ | --------------- |
| Kick         | `●`    | Red             |
| Snare        | `◆`    | Yellow          |
| Closed Hi-hat| `▲`    | Cyan            |
| Open Hi-hat  | `△`    | Blue            |
| Hi-tom       | `■`    | Green           |
| Mid-tom      | `◼`    | Magenta         |
| Low-tom      | `▬`    | White           |
| Cymbal       | `★`    | Bright Yellow   |

#### Hit feedback

- Correct hit: brief flash/highlight on the hit zone in the lane's color
- Miss: brief red flash on the hit zone
- Notes scroll from top to bottom at speed determined by constant pixels-per-second

### 5. Screens / State Machine

```
START -> [CLI arg?] -> MENU (song browser) or LOADING
                              |
MENU -> select song -> LOADING (demucs + analysis)
                              |
LOADING -> complete -> GAMEPLAY
                              |
GAMEPLAY -> ESC -> PAUSED -> ESC -> GAMEPLAY
GAMEPLAY -> song ends -> RESULTS
                              |
RESULTS -> show score + scoreboard -> MENU or QUIT
```

### 6. Scoreboard (`~/.local/share/drum-hero/scores.json`)

```json
{
  "scores": [
    {
      "song": "through_the_fire.mp3",
      "song_hash": "abc123...",
      "player": "default",
      "score": 45200,
      "max_streak": 47,
      "accuracy": 0.89,
      "difficulty": "medium",
      "date": "2026-03-07T14:30:00Z"
    }
  ]
}
```

Results screen shows:
- Final score, max streak, accuracy percentage
- Top 10 scores for this song
- All-time high score

### 7. Cache Structure (`~/.cache/drum-hero/`)

```
~/.cache/drum-hero/
└── {sha256_hash_first16}/
    ├── meta.json          # original filename, hash, separation model used
    ├── drums.wav          # separated drum track
    ├── no_drums.wav       # everything except drums
    └── drummap.json       # analyzed drum hits with timestamps and types
```

---

## Implementation Phases

### Phase 1: Foundation

1. Initialize Go module, set up project structure
2. Config system (XDG paths, TOML parsing, defaults)
3. Cache system (hashing, directory management)
4. Demucs integration (shell out, parse output, cache results)

### Phase 2: Audio Analysis

5. WAV decoder integration (go-audio/wav)
6. Onset detection algorithm (spectral flux)
7. Drum hit classifier (FFT frequency band heuristics)
8. DrumMap serialization (generate + cache `drummap.json`)

### Phase 3: Audio Playback

9. malgo device setup (playback engine)
10. Two-track mixer (no_drums always on, drums muted by default)
11. Real-time drum unmuting on correct hits (atomic signaling)

### Phase 4: Game Engine

12. Game state machine
13. Hit detection logic (timing window, type matching)
14. Scoring system (points, streaks, multiplier)
15. Scoreboard persistence (JSON read/write)

### Phase 5: TUI

16. Bubble Tea app shell, screen routing
17. Song selection menu (CLI arg + directory browser)
18. Loading screen with progress
19. Gameplay screen (falling notes, lanes, hit zone, HUD)
20. Pause overlay
21. Results screen (score, scoreboard display)

### Phase 6: Polish

22. Visual feedback (hit flashes, miss indicators)
23. Smooth note scrolling animation
24. Terminal color theming (respect terminal palette)
25. Error handling, edge cases, graceful degradation
26. Testing

---

## Dependencies Summary

```
go 1.22+

require (
    charm.land/bubbletea/v2    v2.0.1
    charm.land/lipgloss/v2     v2.0.0
    github.com/gen2brain/malgo  v0.11.22
    github.com/go-audio/wav    v1.1.0
    github.com/go-audio/audio  v1.0.0
    github.com/BurntSushi/toml v1.4.0
    github.com/mjibson/go-dsp  v0.0.0   // FFT for spectral analysis
)
```

**External requirements:**
- `demucs` installed and on PATH (`pip install demucs`)
- Terminal with progressive keyboard enhancement support (Alacritty)

---

## Key Design Decisions (Resolved)

| Decision | Choice | Reason |
| --- | --- | --- |
| Language | Go | TUI ecosystem, single binary, performance |
| Demucs integration | Shell out to CLI | Simplest, user installs demucs separately |
| Drum classification | Onset + spectral heuristic | No ML dependency, isolated drum track makes heuristics viable |
| Audio playback | Two-track mixing via malgo | Play no_drums continuously, unmute drums around correct hits |
| TUI layout | Vertical falling notes | Classic Guitar Hero feel, intuitive |
| Caching | XDG cache dir by file hash | Avoid re-processing same songs |
| Song selection | CLI arg + TUI menu | Flexible: quick start or browse |
| Config format | TOML | Human-readable, well-supported in Go |
| Scoreboard format | JSON | Simple, no extra dependencies |
| Note visuals | Color + symbol per drum type | Maximum visual clarity in terminal |
| Difficulty | Named presets + custom ms | Easy onboarding with expert tuning option |

## Default Key Mappings

```
j = kick
f = snare
k = closed-hihat
l = open-hihat
d = hi-tom
s = mid-tom
a = low-tom
; = cymbal
```

Layout rationale: Left hand (a/s/d/f) handles toms and snare, right hand (j/k/l/;) handles kick, hi-hats, and cymbal. This mirrors a drummer's typical left-hand snare / right-hand cymbal orientation.
