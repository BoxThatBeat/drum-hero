# Drum Hero

A Guitar Hero-inspired rhythm game that runs in the terminal, focused on drumming. Feed it any song, and it separates the drums using AI, maps every hit, then challenges you to play along on your keyboard.

```
 ____                        _   _
|  _ \ _ __ _   _ _ __ ___  | | | | ___ _ __ ___
| | | | '__| | | | '_ ' _ \ | |_| |/ _ \ '__/ _ \
| |_| | |  | |_| | | | | | ||  _  |  __/ | | (_) |
|____/|_|   \__,_|_| |_| |_||_| |_|\___|_|  \___/
```

## How It Works

1. You provide any audio file (mp3, wav, flac, etc.)
2. [Demucs](https://github.com/adefossez/demucs) isolates the drum track from the rest of the song
3. Spectral analysis detects every drum hit and classifies it (kick, snare, hi-hat, toms, cymbal)
4. Notes scroll down the screen in 8 color-coded lanes
5. Hit the matching key at the right time -- the drum track only plays when you nail the timing
6. Build streaks, rack up multiplier bonuses, and chase high scores

## Requirements

- **Go 1.22+** (to build)
- **demucs** (for audio separation)
  ```
  pip install demucs
  ```
- A terminal with Unicode support (Alacritty, Kitty, WezTerm, Ghostty, etc.)

### System Dependencies

The audio playback uses [miniaudio](https://miniaud.io/) via CGo. On Linux you need development headers for at least one audio backend:

```bash
# Debian/Ubuntu
sudo apt install libasound2-dev

# Fedora
sudo dnf install alsa-lib-devel

# Arch
sudo pacman -S alsa-lib
```

## Installation

```bash
git clone https://github.com/boxthatbeat/drum-hero.git
cd drum-hero
go build -o drum-hero ./cmd/drum-hero/
```

Or install directly:

```bash
go install github.com/boxthatbeat/drum-hero/cmd/drum-hero@latest
```

## Usage

```
drum-hero                   # Open song selection menu
drum-hero <path-to-song>    # Play a specific song directly
drum-hero --help            # Show help
```

### First Run

On first launch, a default config is created at `~/.config/drum-hero/config.toml`. The menu will scan `~/Music/drum-hero/` for audio files. You can either:

- Drop audio files into `~/Music/drum-hero/` and select from the menu
- Pass any audio file directly: `drum-hero ~/Music/song.mp3`

The first time you play a song, demucs will separate the audio (this takes a while, especially on CPU). Results are cached so subsequent plays load instantly.

## Gameplay

Notes fall from the top of the screen toward the hit zone at the bottom. Each lane represents a drum piece. Press the matching key when the note reaches the hit zone.

```
 Song Name  Score: 4200  Streak: 15  4x         High: 12000
 LWT │ MDT │ HIT │ SNR │ CHH │ OHH │ KCK │ CYM
  a  │  s  │  d  │  f  │  k  │  l  │  j  │  ;
─────────────────────────────────────────────────
     │     │     │     │  ▲  │     │     │
     │     │     │     │     │     │     │
     │  ◼  │     │  ◆  │     │     │  ●  │
     │     │     │     │     │     │     │
     │     │  ■  │     │  ▲  │     │     │  ★
     │     │     │     │     │     │     │
     │     │     │  ◆  │     │     │  ●  │
═════════════════════════════════════════════════
  ▬  │  ◼  │  ■  │  ◆  │  ▲  │  △  │  ●  │  ★
═════════════════════════════════════════════════
 [ESC] Pause
```

### Controls

| Key | Action |
|-----|--------|
| `a` | Low Tom |
| `s` | Mid Tom |
| `d` | Hi Tom |
| `f` | Snare |
| `j` | Kick |
| `k` | Closed Hi-Hat |
| `l` | Open Hi-Hat |
| `;` | Cymbal |
| `Esc` | Pause / Resume |
| `Ctrl+C` | Quit |

Left hand handles toms and snare, right hand handles kick, hi-hats, and cymbal -- mirroring a real drummer's orientation.

### Drum Symbols

| Drum | Symbol | Color |
|------|--------|-------|
| Kick | `●` | Red |
| Snare | `◆` | Yellow |
| Closed Hi-Hat | `▲` | Cyan |
| Open Hi-Hat | `△` | Blue |
| Hi Tom | `■` | Green |
| Mid Tom | `◼` | Magenta |
| Low Tom | `▬` | White |
| Cymbal | `★` | Bright Yellow |

Colors are ANSI 0-15 so they respect your terminal theme.

## Scoring

| Points | Condition |
|--------|-----------|
| 100 | Base points per correct hit |
| x2 | 10-hit streak |
| x4 | 20-hit streak |
| x8 | 30-hit streak (max) |

Points per hit = `100 * multiplier`. Any miss or wrong hit resets the streak and multiplier back to 1x.

At the end of each song you see your final score, max streak, accuracy, and a scoreboard of your top 10 plays for that song. High scores persist across sessions.

## Audio

The drum track is muted by default during playback. When you hit a note on time, the drum track is briefly unmuted around that hit so you hear the actual drum from the recording. Miss a note and there's silence where the drum should be -- you *feel* the gap.

The non-drum audio (vocals, bass, guitar, etc.) plays continuously throughout.

## Configuration

Config file: `~/.config/drum-hero/config.toml`

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
# Options: easy (+/-150ms), medium (+/-100ms), hard (+/-60ms), expert (+/-30ms), custom
preset = "medium"
custom_threshold_ms = 80

[general]
songs_dir = "~/Music/drum-hero"
```

### Difficulty Presets

| Preset | Timing Window | Description |
|--------|---------------|-------------|
| Easy | +/- 150ms | Forgiving, good for learning a song |
| Medium | +/- 100ms | Default, balanced |
| Hard | +/- 60ms | Requires precision |
| Expert | +/- 30ms | Near frame-perfect |

Set `preset = "custom"` and adjust `custom_threshold_ms` for a custom window.

## File Locations

All paths follow the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/).

| Purpose | Path |
|---------|------|
| Config | `~/.config/drum-hero/config.toml` |
| Cache | `~/.cache/drum-hero/<hash>/` |
| Scores | `~/.local/share/drum-hero/scores.json` |

### Cache

Separated tracks and analysis results are cached by SHA256 hash of the input file:

```
~/.cache/drum-hero/<first-16-chars-of-hash>/
  meta.json        # original filename, hash, model used
  drums.wav        # isolated drum track
  no_drums.wav     # everything except drums
  drummap.json     # detected hits with timestamps and classifications
```

Delete the cache directory to force reprocessing.

## How the Analysis Works

### Drum Separation

Uses Meta's [Demucs](https://github.com/adefossez/demucs) `htdemucs_ft` model (fine-tuned Hybrid Transformer) with `--two-stems=drums` to produce a clean drum track and a combined non-drum track.

### Hit Detection

Onset detection via **spectral flux**: computes the FFT of overlapping audio windows, measures the positive change in spectral energy between frames, and applies adaptive thresholding to find transient peaks.

### Hit Classification

Each detected onset is classified by analyzing the frequency band energy distribution of a short window around the hit:

| Drum | Frequency Signature |
|------|-------------------|
| Kick | Dominant energy below 200 Hz |
| Snare | Mid-range (200 Hz - 2 kHz) with broadband noise |
| Closed Hi-Hat | Energy above 5 kHz, fast decay |
| Open Hi-Hat | Energy above 5 kHz, slower decay |
| Hi Tom | Peak around 300 - 600 Hz |
| Mid Tom | Peak around 150 - 350 Hz |
| Low Tom | Peak around 80 - 200 Hz |
| Cymbal | Broad energy above 3 kHz, long sustain |

Since demucs already isolates the drums, these heuristics work well without needing a machine learning model.

## Project Structure

```
cmd/drum-hero/main.go           Entry point, CLI
internal/
  config/config.go               XDG config, key mappings, difficulty
  cache/cache.go                 SHA256 hashing, cache management
  audio/
    demucs.go                    Demucs CLI integration
    decoder.go                   WAV decoding (go-audio/wav)
    player.go                    Real-time playback + two-track mixer (malgo)
  analysis/
    onset.go                     Spectral flux onset detection
    classifier.go                FFT frequency band classification
    drummap.go                   DrumMap type + JSON serialization
  game/
    engine.go                    Hit detection, note tracking
    scoring.go                   Points, streaks, multipliers
    state.go                     Game state machine
  score/scoreboard.go            JSON scoreboard persistence
  tui/
    app.go                       Bubble Tea app shell, screen routing
    menu.go                      Song selection browser
    loading.go                   Processing progress screen
    gameplay.go                  Falling notes, lanes, hit zone, HUD
    results.go                   End-of-song score display
    styles.go                    Colors, symbols, layout constants
```

## Tech Stack

| Component | Library |
|-----------|---------|
| TUI | [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) |
| Styling | [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) |
| Audio | [malgo](https://github.com/gen2brain/malgo) (miniaudio) |
| WAV | [go-audio/wav](https://github.com/go-audio/wav) |
| FFT | [go-dsp](https://github.com/madelynnblue/go-dsp) |
| Config | [BurntSushi/toml](https://github.com/BurntSushi/toml) |
| Separation | [Demucs](https://github.com/adefossez/demucs) (external) |

## License

MIT
