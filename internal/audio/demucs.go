package audio

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boxthatbeat/drum-hero/internal/cache"
)

const (
	demucsModel = "htdemucs_ft"
	demucsBin   = "demucs"
)

// ProgressFunc is called with progress messages during separation.
type ProgressFunc func(msg string)

// findDemucs locates the demucs binary by checking:
// 1. PATH (standard lookup)
// 2. A venv/bin directory next to the running executable
func findDemucs() (string, error) {
	// Check PATH first
	if path, err := exec.LookPath(demucsBin); err == nil {
		return path, nil
	}

	// Check for a venv next to the executable
	if exe, err := os.Executable(); err == nil {
		venvPath := filepath.Join(filepath.Dir(exe), "venv", "bin", demucsBin)
		if _, err := os.Stat(venvPath); err == nil {
			return venvPath, nil
		}
	}

	// Check for a venv in the current working directory
	if wd, err := os.Getwd(); err == nil {
		venvPath := filepath.Join(wd, "venv", "bin", demucsBin)
		if _, err := os.Stat(venvPath); err == nil {
			return venvPath, nil
		}
	}

	return "", fmt.Errorf("demucs not found on PATH or in ./venv: install with 'pip install demucs'")
}

// CheckDemucs verifies that demucs is installed and accessible.
func CheckDemucs() error {
	_, err := findDemucs()
	return err
}

// Separate runs demucs to separate a song into drums and no_drums tracks.
// It caches the results keyed by file hash. If already cached, returns immediately.
// The onProgress callback receives status messages for UI display.
func Separate(audioPath string, onProgress ProgressFunc) (hash string, err error) {
	if onProgress == nil {
		onProgress = func(string) {}
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(audioPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	// Verify file exists
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("audio file not found: %w", err)
	}

	// Hash the file
	onProgress("Hashing audio file...")
	hash, err = cache.HashFile(absPath)
	if err != nil {
		return "", err
	}

	// Check cache
	if cache.HasSeparation(hash) {
		onProgress("Using cached separation")
		return hash, nil
	}

	// Check demucs is available
	demucsPath, err := findDemucs()
	if err != nil {
		return "", err
	}

	// Create a temp output directory for demucs
	tmpDir, err := os.MkdirTemp("", "drum-hero-demucs-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Run demucs
	onProgress(fmt.Sprintf("Separating with %s model (this may take a while)...", demucsModel))

	cmd := exec.Command(demucsPath,
		"-n", demucsModel,
		"--two-stems=drums",
		"-o", tmpDir,
		absPath,
	)

	// Capture stderr for progress (demucs writes progress to stderr)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Also capture stdout
	cmd.Stdout = io.Discard

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting demucs: %w", err)
	}

	// Stream stderr for progress
	scanner := bufio.NewScanner(stderrPipe)
	scanner.Split(scanLines)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			onProgress(line)
		}
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("demucs failed: %w", err)
	}

	// Find the output files
	// demucs outputs to: <outdir>/<model>/<songname>/drums.wav and no_drums.wav
	songName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	demucsOutDir := filepath.Join(tmpDir, demucsModel, songName)

	drumsFile := filepath.Join(demucsOutDir, "drums.wav")
	noDrumsFile := filepath.Join(demucsOutDir, "no_drums.wav")

	// Verify output files exist
	if _, err := os.Stat(drumsFile); err != nil {
		return "", fmt.Errorf("demucs did not produce drums.wav: %w", err)
	}
	if _, err := os.Stat(noDrumsFile); err != nil {
		return "", fmt.Errorf("demucs did not produce no_drums.wav: %w", err)
	}

	// Move to cache
	onProgress("Caching separated tracks...")
	if err := cache.EnsureDir(hash); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}

	if err := copyFile(drumsFile, cache.DrumsPath(hash)); err != nil {
		return "", fmt.Errorf("caching drums: %w", err)
	}
	if err := copyFile(noDrumsFile, cache.NoDrumsPath(hash)); err != nil {
		return "", fmt.Errorf("caching no_drums: %w", err)
	}

	// Write metadata
	meta := cache.Meta{
		OriginalFile: absPath,
		SHA256:       hash,
		Model:        demucsModel,
	}
	if err := cache.WriteMeta(hash, meta); err != nil {
		return "", fmt.Errorf("writing meta: %w", err)
	}

	onProgress("Separation complete")
	return hash, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

// scanLines is a split function for bufio.Scanner that handles \r\n, \n, and \r (for progress bars).
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Find the earliest line terminator: \r\n, \n, or \r
	cr := bytes.IndexByte(data, '\r')
	lf := bytes.IndexByte(data, '\n')

	switch {
	case cr >= 0 && cr+1 < len(data) && data[cr+1] == '\n':
		// \r\n found
		return cr + 2, data[0:cr], nil
	case cr >= 0 && (lf < 0 || cr < lf):
		// \r found before \n (or no \n)
		return cr + 1, data[0:cr], nil
	case lf >= 0:
		// \n found
		return lf + 1, data[0:lf], nil
	}

	// If at EOF, deliver what's left
	if atEOF {
		return len(data), data, nil
	}

	// Request more data
	return 0, nil, nil
}
