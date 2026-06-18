package render

const (
	glyphFull         = "▀" // upper half block: fg=top pixel, bg=bottom pixel
	glyphOwnCursor    = "+"
	glyphRemoteTop    = "🮎" // U+1FB8E upper half medium shade
	glyphRemoteBottom = "🮏" // U+1FB8F lower half medium shade
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
// Precedence: own cursor > remote cursor > normal half-block.
//   - top, bottom: the two pixel color indices in this cell.
//   - ownHere: the local user's cursor is in this cell; ownColor is their color.
//   - remoteHere: a remote cursor is in this cell; remoteOnTop picks the half;
//     remoteColor is that user's color.
func DecideCell(top, bottom int, ownHere bool, ownColor int, remoteHere, remoteOnTop bool, remoteColor int) CellSpec {
	switch {
	case ownHere:
		return CellSpec{Glyph: glyphOwnCursor, FG: ownColor, BG: bottom}
	case remoteHere && remoteOnTop:
		return CellSpec{Glyph: glyphRemoteTop, FG: remoteColor, BG: bottom}
	case remoteHere && !remoteOnTop:
		return CellSpec{Glyph: glyphRemoteBottom, FG: remoteColor, BG: top}
	default:
		return CellSpec{Glyph: glyphFull, FG: top, BG: bottom}
	}
}
