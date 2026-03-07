package tui

import "github.com/boxthatbeat/drum-hero/internal/cache"

// cachePkg wraps the cache package to avoid import naming issues.
var cachePkg = struct {
	NoDrumsPath func(string) string
	DrumsPath   func(string) string
}{
	NoDrumsPath: cache.NoDrumsPath,
	DrumsPath:   cache.DrumsPath,
}
