package render

import (
	"strings"
	"testing"
)

func TestRenderDimensions(t *testing.T) {
	// 4x4 canvas, viewport showing all of it: 4 pixel cols, 4 pixel rows -> 2 cell rows.
	grid := make([]byte, 16)
	v := View{
		Grid: grid, Width: 4, Height: 4,
		CamX: 0, CamY: 0, PixelCols: 4, PixelRows: 4,
		CursorX: 0, CursorY: 0, SelectedColor: 1,
		Remotes: nil,
	}
	out := Canvas(v)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d rows, want 2", len(lines))
	}
}

func TestRenderPlacesOwnCursorGlyph(t *testing.T) {
	grid := make([]byte, 16)
	v := View{
		Grid: grid, Width: 4, Height: 4,
		CamX: 0, CamY: 0, PixelCols: 4, PixelRows: 4,
		CursorX: 0, CursorY: 0, SelectedColor: 1,
	}
	out := Canvas(v)
	if !strings.Contains(out, "+") {
		t.Error("expected own-cursor glyph '+' in output")
	}
}

func TestRenderPlacesRemoteGlyph(t *testing.T) {
	grid := make([]byte, 16)
	v := View{
		Grid: grid, Width: 4, Height: 4,
		CamX: 0, CamY: 0, PixelCols: 4, PixelRows: 4,
		CursorX: 3, CursorY: 3, SelectedColor: 1,
		Remotes: []RemoteCursor{{X: 1, Y: 0, Color: 2}},
	}
	out := Canvas(v)
	if !strings.Contains(out, "🮎") {
		t.Error("expected remote top glyph 🮎 in output")
	}
}
