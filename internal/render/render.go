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

// CellStyler turns a CellSpec into its styled terminal string, caching the
// result per spec. There are at most a few hundred distinct specs (4 glyphs ×
// 8 foregrounds × 8 backgrounds), so after warm-up every cell is a map lookup
// instead of an expensive lipgloss render — the difference between rebuilding
// thousands of styles per frame and not.
type CellStyler struct {
	r     *lipgloss.Renderer
	cache map[CellSpec]string
}

// NewCellStyler returns a styler bound to a per-session renderer (from
// bubbletea.MakeRenderer, carrying the client's color profile). A nil renderer
// falls back to the global default (which reports no color outside a TTY).
func NewCellStyler(r *lipgloss.Renderer) *CellStyler {
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	return &CellStyler{r: r, cache: make(map[CellSpec]string)}
}

// Style returns the styled string for spec, populating the cache on first use.
func (s *CellStyler) Style(spec CellSpec) string {
	if v, ok := s.cache[spec]; ok {
		return v
	}
	out := s.r.NewStyle().
		Foreground(ColorAt(spec.FG)).
		Background(ColorAt(spec.BG)).
		Render(spec.Glyph)
	s.cache[spec] = out
	return out
}

// View is everything Canvas needs to render the visible viewport.
type View struct {
	// Styler styles each cell and carries the client's color profile. If nil, a
	// default styler is created per call (uncached across frames).
	Styler *CellStyler

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
	styler := v.Styler
	if styler == nil {
		styler = NewCellStyler(nil)
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
			b.WriteString(styler.Style(spec))
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
