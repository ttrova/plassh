package render

import "testing"

func TestDecideCellNormal(t *testing.T) {
	got := DecideCell(3, 5, false, 0, false, false, 0)
	want := CellSpec{Glyph: "▀", FG: 3, BG: 5}
	if got != want {
		t.Errorf("normal: got %+v, want %+v", got, want)
	}
}

func TestDecideCellOwnCursorWins(t *testing.T) {
	// Own cursor present along with a remote cursor: own wins.
	got := DecideCell(3, 5, true, 2, true, true, 6)
	want := CellSpec{Glyph: "+", FG: 2, BG: 5}
	if got != want {
		t.Errorf("own: got %+v, want %+v", got, want)
	}
}

func TestDecideCellRemoteTop(t *testing.T) {
	got := DecideCell(3, 5, false, 0, true, true, 6)
	want := CellSpec{Glyph: "🮎", FG: 6, BG: 5}
	if got != want {
		t.Errorf("remote top: got %+v, want %+v", got, want)
	}
}

func TestDecideCellRemoteBottom(t *testing.T) {
	got := DecideCell(3, 5, false, 0, true, false, 6)
	want := CellSpec{Glyph: "🮏", FG: 6, BG: 3}
	if got != want {
		t.Errorf("remote bottom: got %+v, want %+v", got, want)
	}
}
