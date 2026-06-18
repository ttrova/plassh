// Package presence tracks live cursor positions, colors, and usernames in Redis.
package presence

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const maxNameLen = 16

// Update describes a remote user's cursor state, or their departure (Gone).
type Update struct {
	ID    string
	X     int
	Y     int
	Color int
	Name  string
	Gone  bool
}

// Encode renders the update as a pipe-delimited payload. Name is last so it may
// contain commas; pipes are stripped by SanitizeName before this is ever called.
//
//	move/join: "id|x|y|color|name"
//	gone:      "id|gone"
func (u Update) Encode() string {
	if u.Gone {
		return u.ID + "|gone"
	}
	return fmt.Sprintf("%s|%d|%d|%d|%s", u.ID, u.X, u.Y, u.Color, u.Name)
}

// Decode parses a payload produced by Encode.
func Decode(payload string) (Update, error) {
	parts := strings.SplitN(payload, "|", 5)
	if len(parts) == 2 && parts[1] == "gone" {
		return Update{ID: parts[0], Gone: true}, nil
	}
	if len(parts) != 5 {
		return Update{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}
	x, err := strconv.Atoi(parts[1])
	if err != nil {
		return Update{}, fmt.Errorf("x: %w", err)
	}
	y, err := strconv.Atoi(parts[2])
	if err != nil {
		return Update{}, fmt.Errorf("y: %w", err)
	}
	color, err := strconv.Atoi(parts[3])
	if err != nil {
		return Update{}, fmt.Errorf("color: %w", err)
	}
	return Update{ID: parts[0], X: x, Y: y, Color: color, Name: parts[4]}, nil
}

// SanitizeName strips control characters and pipes, trims whitespace, caps the
// length, and falls back to "anon" for empty input. Used for display safety.
func SanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '|' || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > maxNameLen {
		out = out[:maxNameLen]
	}
	if out == "" {
		return "anon"
	}
	return out
}
