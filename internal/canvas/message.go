// Package canvas stores the pixel grid in Redis and broadcasts pixel changes.
package canvas

import (
	"fmt"
	"strconv"
	"strings"
)

// PixelUpdate is a single pixel change broadcast over the updates channel.
type PixelUpdate struct {
	X     int
	Y     int
	Color int
}

// Index returns the flat byte offset of pixel (x, y) in a grid of the given width.
func Index(x, y, width int) int {
	return y*width + x
}

// Encode renders the update as a compact "x,y,color" payload.
func (u PixelUpdate) Encode() string {
	return fmt.Sprintf("%d,%d,%d", u.X, u.Y, u.Color)
}

// Decode parses a payload produced by Encode.
func Decode(payload string) (PixelUpdate, error) {
	parts := strings.Split(payload, ",")
	if len(parts) != 3 {
		return PixelUpdate{}, fmt.Errorf("expected 3 fields, got %d", len(parts))
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return PixelUpdate{}, fmt.Errorf("field %d: %w", i, err)
		}
		nums[i] = n
	}
	return PixelUpdate{X: nums[0], Y: nums[1], Color: nums[2]}, nil
}
