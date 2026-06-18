package render

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Regression test: when a renderer with a real color profile is supplied, Canvas
// must emit ANSI escape sequences. The original bug used the global lipgloss
// renderer, which detects "no color" off a TTY (e.g. inside a container) and
// stripped all styling regardless of the client's terminal.
func TestCanvasEmitsColorWithRenderer(t *testing.T) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI256)

	grid := make([]byte, 16)
	grid[0] = 1 // a red pixel at (0,0)
	out := Canvas(View{
		Renderer: r,
		Grid:     grid, Width: 4, Height: 4,
		PixelCols: 4, PixelRows: 4,
		CursorX: 99, CursorY: 99, // own cursor off-canvas
		SelectedColor: 1,
	})

	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI escape sequences in colored output, got %q", out)
	}
}
