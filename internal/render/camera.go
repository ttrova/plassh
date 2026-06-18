package render

// Chrome dimensions: a 1-row player-list header, a 1-cell border on each side,
// and a 1-row status bar.
const (
	borderRows = 2 // top + bottom border
	borderCols = 2 // left + right border
	statusRows = 1
	headerRows = 1 // connected-players line above the canvas
)

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// CameraFor returns the new camera origin (in pixels, one axis) such that the
// cursor stays visible within a viewport of the given size, clamped so the view
// never scrolls past the canvas. canvasSize is the canvas extent on that axis.
func CameraFor(cursor, cam, viewport, canvasSize int) int {
	if cursor < cam {
		cam = cursor
	} else if cursor >= cam+viewport {
		cam = cursor - viewport + 1
	}
	maxCam := canvasSize - viewport
	if maxCam < 0 {
		maxCam = 0
	}
	return clamp(cam, 0, maxCam)
}

// VisiblePixelHeight converts a terminal row count into visible pixel rows,
// accounting for chrome and the 2-pixels-per-cell vertical doubling.
func VisiblePixelHeight(termRows int) int {
	usable := termRows - borderRows - statusRows - headerRows
	if usable < 0 {
		usable = 0
	}
	return usable * 2
}

// VisiblePixelWidth converts a terminal column count into visible pixel columns.
func VisiblePixelWidth(termCols int) int {
	usable := termCols - borderCols
	if usable < 0 {
		usable = 0
	}
	return usable
}
