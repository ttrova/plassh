package render

import "hash/fnv"

// glyphFull is the normal half-block cell: fg = top pixel, bg = bottom pixel.
const glyphFull = "▀" // U+2580 upper half block

// cursorStyles are the per-player cursor glyph pairs {topHalf, bottomHalf}. Each
// pair reads as a top- or bottom-anchored mark in the player's color (and is
// distinct from the solid ▀ used for painted pixels). Players are assigned a
// style from this pool so cursors are distinguishable beyond color alone.
var cursorStyles = [][2]string{
	{string(rune(0x1FB8E)), string(rune(0x1FB8F))}, // 🮎/🮏 medium-shade half
	{string(rune(0x1FB82)), string(rune(0x2582))},  // 🮂/▂ one-quarter
	{string(rune(0x1FB85)), string(rune(0x2586))},  // 🮅/▆ three-quarter
	{string(rune(0x2594)), string(rune(0x2581))},   // ▔/▁ one-eighth edge
}

// StyleFor maps a session id to a cursor style index, deterministically so every
// client renders the same player with the same style.
func StyleFor(id string) int {
	if id == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return int(h.Sum32()) % len(cursorStyles)
}

func cursorGlyph(style int, onTop bool) string {
	pair := cursorStyles[((style%len(cursorStyles))+len(cursorStyles))%len(cursorStyles)]
	if onTop {
		return pair[0]
	}
	return pair[1]
}

// Cursor describes a cursor that may occupy a cell.
type Cursor struct {
	Here  bool
	OnTop bool // true if on the cell's top pixel, false if bottom
	Color int
	Style int
}

// CellSpec is the resolved render decision for one terminal cell: which glyph to
// draw and which palette indices to use for foreground and background.
type CellSpec struct {
	Glyph string
	FG    int
	BG    int
}

// DecideCell resolves one cell from the underlying two pixels plus cursor state.
//
// Both the local cursor and remote cursors render as a top/bottom half glyph (in
// that cursor's style and color); the cell's other half keeps its real pixel
// color. Precedence: own cursor > remote cursor > normal half-block.
func DecideCell(top, bottom int, own, remote Cursor) CellSpec {
	switch {
	case own.Here:
		return cursorCell(own, top, bottom)
	case remote.Here:
		return cursorCell(remote, top, bottom)
	default:
		return CellSpec{Glyph: glyphFull, FG: top, BG: bottom}
	}
}

func cursorCell(c Cursor, top, bottom int) CellSpec {
	bg := top // bottom-pixel cursor shows the real top pixel behind it
	if c.OnTop {
		bg = bottom
	}
	return CellSpec{Glyph: cursorGlyph(c.Style, c.OnTop), FG: c.Color, BG: bg}
}
