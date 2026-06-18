package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RemoteCursor is one other user's cursor position and color (pixel coords).
type RemoteCursor struct {
	X     int
	Y     int
	Color int
}

// View is everything Canvas needs to render the visible viewport.
type View struct {
	// Renderer is the per-session lipgloss renderer (from bubbletea.MakeRenderer)
	// that carries the client's color profile. If nil, the global default
	// renderer is used (which reports no color outside a TTY).
	Renderer *lipgloss.Renderer

	Grid          []byte
	Width         int
	Height        int
	CamX          int
	CamY          int
	PixelCols     int
	PixelRows     int // must be even; the visible pixel height
	CursorX       int
	CursorY       int
	SelectedColor int
	Remotes       []RemoteCursor
}

// Canvas renders the visible region as a string of styled cells, one terminal
// row per two pixel rows. It does not include border or status bar.
func Canvas(v View) string {
	r := v.Renderer
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	cellRows := v.PixelRows / 2
	var b strings.Builder
	for row := 0; row < cellRows; row++ {
		for col := 0; col < v.PixelCols; col++ {
			px := v.CamX + col
			topY := v.CamY + row*2
			botY := topY + 1

			top := v.pixel(px, topY)
			bottom := v.pixel(px, botY)

			ownTop := px == v.CursorX && topY == v.CursorY
			ownBot := px == v.CursorX && botY == v.CursorY
			ownHere := ownTop || ownBot

			remoteHere, remoteOnTop, remoteColor := v.remoteAt(px, topY, botY)

			spec := DecideCell(top, bottom, ownHere, ownTop, v.SelectedColor, remoteHere, remoteOnTop, remoteColor)
			style := r.NewStyle().
				Foreground(ColorAt(spec.FG)).
				Background(ColorAt(spec.BG))
			b.WriteString(style.Render(spec.Glyph))
		}
		if row < cellRows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// pixel returns the color index at (x, y), or 0 (black) if out of bounds.
func (v View) pixel(x, y int) int {
	if x < 0 || y < 0 || x >= v.Width || y >= v.Height {
		return 0
	}
	return int(v.Grid[y*v.Width+x])
}

// remoteAt reports whether a remote cursor sits on either pixel of this cell.
func (v View) remoteAt(px, topY, botY int) (here, onTop bool, color int) {
	for _, r := range v.Remotes {
		if r.X != px {
			continue
		}
		if r.Y == topY {
			return true, true, r.Color
		}
		if r.Y == botY {
			return true, false, r.Color
		}
	}
	return false, false, 0
}
