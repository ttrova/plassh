// Package render turns canvas state into a styled terminal string.
package render

import (
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

// NumColors is the size of the MVP palette (standard ANSI 0-7).
const NumColors = 8

var colorNames = [NumColors]string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
}

// ColorName returns the human-readable name of a color index (0-7).
func ColorName(c int) string {
	if c < 0 || c >= NumColors {
		return "?"
	}
	return colorNames[c]
}

// ColorAt returns the lipgloss color for a color index, mapping to ANSI 0-7.
func ColorAt(c int) lipgloss.Color {
	if c < 0 || c >= NumColors {
		c = 0
	}
	return lipgloss.Color(strconv.Itoa(c))
}

// NextColor returns the next palette index, wrapping from 7 back to 0.
func NextColor(c int) int {
	return (c + 1) % NumColors
}

// PrevColor returns the previous palette index, wrapping from 0 back to 7.
func PrevColor(c int) int {
	return (c - 1 + NumColors) % NumColors
}
