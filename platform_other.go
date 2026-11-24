//go:build !linux

package main

import "github.com/wailsapp/wails/v2/pkg/options"

// applyPlatformOptions leaves the shared options untouched on macOS and Windows.
func applyPlatformOptions(*options.App) {}
