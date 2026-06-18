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

// EncodeBatch renders many updates as a single payload: "x,y,c;x,y,c;...". A
// batch is broadcast as one message so a bulk operation doesn't flood pub/sub.
func EncodeBatch(ups []PixelUpdate) string {
	parts := make([]string, len(ups))
	for i, u := range ups {
		parts[i] = u.Encode()
	}
	return strings.Join(parts, ";")
}

// Decode parses a payload produced by Encode or EncodeBatch into one or more
// updates.
func Decode(payload string) ([]PixelUpdate, error) {
	items := strings.Split(payload, ";")
	out := make([]PixelUpdate, 0, len(items))
	for _, item := range items {
		parts := strings.Split(item, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("expected 3 fields, got %d", len(parts))
		}
		nums := make([]int, 3)
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil, fmt.Errorf("field %d: %w", i, err)
			}
			nums[i] = n
		}
		out = append(out, PixelUpdate{X: nums[0], Y: nums[1], Color: nums[2]})
	}
	return out, nil
}
