package render

const (
	glyphFull         = "▀" // upper half block: fg=top pixel, bg=bottom pixel
	glyphCursorTop    = "🮎" // U+1FB8E upper half medium shade
	glyphCursorBottom = "🮏" // U+1FB8F lower half medium shade
)

// CellSpec is the resolved render decision for one terminal cell: which glyph to
// draw and which palette indices to use for foreground and background.
type CellSpec struct {
	Glyph string
	FG    int
	BG    int
}

// DecideCell resolves one cell from the underlying two pixels plus cursor state.
//
// Both the local cursor and remote cursors render as a top/bottom hatched square
// (medium-shade half block) in that user's color; the cell's other half keeps its
// real pixel color. Precedence: own cursor > remote cursor > normal half-block.
//   - top, bottom: the two pixel color indices in this cell.
//   - ownHere/ownOnTop: the local cursor is in this cell, on the top or bottom
//     pixel; ownColor is the selected color.
//   - remoteHere/remoteOnTop: a remote cursor is in this cell, on the top or
//     bottom pixel; remoteColor is that user's color.
func DecideCell(top, bottom int, ownHere, ownOnTop bool, ownColor int, remoteHere, remoteOnTop bool, remoteColor int) CellSpec {
	switch {
	case ownHere && ownOnTop:
		return CellSpec{Glyph: glyphCursorTop, FG: ownColor, BG: bottom}
	case ownHere && !ownOnTop:
		return CellSpec{Glyph: glyphCursorBottom, FG: ownColor, BG: top}
	case remoteHere && remoteOnTop:
		return CellSpec{Glyph: glyphCursorTop, FG: remoteColor, BG: bottom}
	case remoteHere && !remoteOnTop:
		return CellSpec{Glyph: glyphCursorBottom, FG: remoteColor, BG: top}
	default:
		return CellSpec{Glyph: glyphFull, FG: top, BG: bottom}
	}
}
