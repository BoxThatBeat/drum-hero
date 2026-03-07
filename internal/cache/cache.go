package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Meta holds metadata about a cached song separation.
type Meta struct {
	OriginalFile string `json:"original_file"`
	SHA256       string `json:"sha256"`
	Model        string `json:"model"`
}

// cacheDir returns the XDG cache directory for drum-hero.
func cacheDir() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "drum-hero")
}

// HashFile computes the SHA256 hash of a file, returning the hex-encoded string.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Dir returns the cache directory for a given file hash.
// Uses the first 16 characters of the hash as the directory name.
func Dir(hash string) string {
	key := hash
	if len(key) > 16 {
		key = key[:16]
	}
	return filepath.Join(cacheDir(), key)
}

// Exists checks if a cache entry exists for the given hash.
func Exists(hash string) bool {
	dir := Dir(hash)
	metaPath := filepath.Join(dir, "meta.json")
	_, err := os.Stat(metaPath)
	return err == nil
}

// DrumsPath returns the path to the cached drums.wav for a given hash.
func DrumsPath(hash string) string {
	return filepath.Join(Dir(hash), "drums.wav")
}

// NoDrumsPath returns the path to the cached no_drums.wav for a given hash.
func NoDrumsPath(hash string) string {
	return filepath.Join(Dir(hash), "no_drums.wav")
}

// DrumMapPath returns the path to the cached drummap.json for a given hash.
func DrumMapPath(hash string) string {
	return filepath.Join(Dir(hash), "drummap.json")
}

// MetaPath returns the path to the cached meta.json for a given hash.
func MetaPath(hash string) string {
	return filepath.Join(Dir(hash), "meta.json")
}

// HasDrumMap checks if a drum map analysis has been cached for the given hash.
func HasDrumMap(hash string) bool {
	_, err := os.Stat(DrumMapPath(hash))
	return err == nil
}

// HasSeparation checks if demucs separation has been cached for the given hash.
func HasSeparation(hash string) bool {
	_, errD := os.Stat(DrumsPath(hash))
	_, errN := os.Stat(NoDrumsPath(hash))
	return errD == nil && errN == nil
}

// EnsureDir creates the cache directory for a given hash if it doesn't exist.
func EnsureDir(hash string) error {
	return os.MkdirAll(Dir(hash), 0o755)
}

// WriteMeta writes the meta.json file for a cache entry.
func WriteMeta(hash string, meta Meta) error {
	if err := EnsureDir(hash); err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling meta: %w", err)
	}

	return os.WriteFile(MetaPath(hash), data, 0o644)
}

// ReadMeta reads the meta.json file for a cache entry.
func ReadMeta(hash string) (Meta, error) {
	var meta Meta
	data, err := os.ReadFile(MetaPath(hash))
	if err != nil {
		return meta, fmt.Errorf("reading meta: %w", err)
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("parsing meta: %w", err)
	}

	return meta, nil
}
