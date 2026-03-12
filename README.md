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
3. Spectral analysis detects every drum hit and classifies it (kick, snare, hi-hat, cymbal)
4. Notes scroll down the screen in 5 color-coded lanes
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
go build -o drum-hero .
```

Or install directly:

```bash
go install github.com/boxthatbeat/drum-hero@latest
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
 CHH │ OHH │ SNR │ KCK │ CYM
  a  │  s  │  d  │  j  │  k
─────────────────────────────────
     │     │     │     │
     │     │  ◆  │  ●  │
     │     │     │     │  ★
  ▲  │     │     │     │
     │     │  ◆  │  ●  │
     │  △  │     │     │
═════════════════════════════════
  ▲  │  △  │  ◆  │  ●  │  ★
═════════════════════════════════
 [ESC] Pause
```

### Controls

| Key | Action |
|-----|--------|
| `a` | Closed Hi-Hat |
| `s` | Open Hi-Hat |
| `d` | Snare |
| `j` | Kick |
| `k` | Cymbal |
| `Esc` | Pause / Resume |
| `Ctrl+C` | Quit |

Left hand handles hi-hats and snare, right hand handles kick and cymbal.

### Drum Symbols

| Drum | Symbol | Color |
|------|--------|-------|
| Kick | `●` | Red |
| Snare | `◆` | Yellow |
| Closed Hi-Hat | `▲` | Cyan |
| Open Hi-Hat | `△` | Blue |
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
closed-hihat = "a"
open-hihat = "s"
snare = "d"
kick = "j"
cymbal = "k"

[difficulty]
# Options: easy (+/-150ms), medium (+/-100ms), hard (+/-60ms), expert (+/-30ms), custom
preset = "medium"
custom_threshold_ms = 80

[audio]
# How long (ms) the drum track stays audible after a correct hit
drum_unmute_ms = 300

[classifier]
# Thresholds for drum hit classification (see Tuning section below)
kick_threshold = 0.50
hihat_threshold = 0.20
snare_bands = 4
simultaneous_low = 0.30
simultaneous_high = 0.15

# Frequency band boundaries (Hz) — see "Frequency Band Boundaries" section below
freq_sub_bass = 80
freq_bass = 200
freq_low_mid = 600
freq_mid = 2000
freq_high_mid = 5000
freq_high = 10000

# Onset detection parameters
onset_fft_size = 2048
onset_hop_size = 512
onset_threshold = 0.30
onset_median_window = 7

# Minimum interval (ms) between consecutive hits per drum type
min_interval_kick_ms = 30
min_interval_snare_ms = 30
min_interval_closedhh_ms = 30
min_interval_openhh_ms = 50
min_interval_cymbal_ms = 50

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
| Snare | Broadband energy across 4+ frequency bands with mid-range body |
| Closed Hi-Hat | Energy above 2 kHz, fast decay |
| Open Hi-Hat | Energy above 2 kHz, slower decay |
| Cymbal | Broad energy above 5 kHz, long sustain |

When two drums are played at the same time (e.g. kick + hi-hat), the classifier detects both and produces two notes at the same timestamp, requiring both keys to be pressed.

Since demucs already isolates the drums, these heuristics work well without needing a machine learning model.

### Tuning the Classifier

The classifier thresholds are configurable in `[classifier]` and can be adjusted per-song style. When you change any threshold, cached drum maps are automatically invalidated and re-analyzed on the next play.

| Setting | Default | What it controls |
|---------|---------|-----------------|
| `kick_threshold` | 0.50 | Minimum low-frequency energy ratio (sub-bass + bass) to classify as kick. Lower = more kicks detected. |
| `hihat_threshold` | 0.20 | Minimum high-frequency energy ratio (high-mid + high + very-high) to classify as hi-hat/cymbal. Lower = more hi-hats and cymbals detected. |
| `snare_bands` | 4 | Minimum number of frequency bands (out of 7) with significant energy for broadband snare detection. The snare check runs *before* the hi-hat check, so raising this lets more hits fall through to hi-hat/cymbal. |
| `simultaneous_low` | 0.30 | Low-frequency threshold for detecting simultaneous kick + hi-hat/cymbal hits. |
| `simultaneous_high` | 0.15 | High-frequency threshold for detecting simultaneous kick + hi-hat/cymbal hits. |

**Common adjustments:**

- **Not enough hi-hats/cymbals?** Lower `hihat_threshold` (try `0.10`) and/or raise `snare_bands` to `5`.
- **Too many false kicks?** Raise `kick_threshold` (try `0.60`).
- **Snare eating everything?** Raise `snare_bands` to `5` or `6` to require a wider spectral spread before classifying as snare.
- **Missing simultaneous hits?** Lower `simultaneous_high` (try `0.10`) to detect hi-hats layered with kicks more easily.

### Frequency Band Boundaries

The classifier splits the spectrum into 7 energy bands. The boundaries between these bands are also configurable:

| Setting | Default | Band it defines |
|---------|---------|----------------|
| `freq_sub_bass` | 80 | SubBass: 20 Hz to this value |
| `freq_bass` | 200 | Bass: sub_bass to this value |
| `freq_low_mid` | 600 | LowMid: bass to this value |
| `freq_mid` | 2000 | Mid: low_mid to this value |
| `freq_high_mid` | 5000 | HighMid: mid to this value |
| `freq_high` | 10000 | High: high_mid to this value; VeryHigh is everything above |

Hi-hat and cymbal detection uses the **HighMid + High + VeryHigh** bands. If hi-hats still aren't being detected even with a low `hihat_threshold`, the energy is likely landing in the Mid band instead. Try lowering `freq_mid` (e.g. from `2000` to `1000`) to shift that energy into HighMid where it will count toward hi-hat classification.

### Onset Detection

These control how the spectral flux algorithm finds drum hit transients:

| Setting | Default | What it controls |
|---------|---------|-----------------|
| `onset_fft_size` | 2048 | FFT window size in samples. Larger = better frequency resolution but worse time resolution. |
| `onset_hop_size` | 512 | Samples between analysis frames. Smaller = finer time resolution but slower. |
| `onset_threshold` | 0.30 | Minimum spectral flux to count as an onset. Lower = more sensitive (more hits detected, more false positives). |
| `onset_median_window` | 7 | Adaptive threshold median window size. Larger = smoother threshold, may miss rapid hits. |

### Per-Drum-Type Minimum Intervals

These set the minimum time (ms) allowed between consecutive hits of the **same drum type**. If two hits of the same type are closer than this, the second is dropped. Different drums need different intervals — kicks can have fast double hits, while cymbals naturally ring longer.

| Setting | Default | Drum |
|---------|---------|------|
| `min_interval_kick_ms` | 30 | Kick |
| `min_interval_snare_ms` | 30 | Snare |
| `min_interval_closedhh_ms` | 30 | Closed hi-hat |
| `min_interval_openhh_ms` | 50 | Open hi-hat |
| `min_interval_cymbal_ms` | 50 | Cymbal |

**Common adjustments:**

- **Fast double kick not showing both hits?** Lower `min_interval_kick_ms` to `20` or even `15`.
- **Fast snare roll missing notes?** Lower `min_interval_snare_ms`.
- **Too many spurious notes?** Raise `onset_threshold` (try `0.40`) or raise the per-type intervals.
- **Missing quiet ghost notes?** Lower `onset_threshold` (try `0.15`).

## Project Structure

```
main.go                         Entry point, CLI
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
