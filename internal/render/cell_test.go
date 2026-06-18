package render

import "testing"

func TestDecideCellNormal(t *testing.T) {
	got := DecideCell(3, 5, false, false, 0, false, false, 0)
	want := CellSpec{Glyph: "▀", FG: 3, BG: 5}
	if got != want {
		t.Errorf("normal: got %+v, want %+v", got, want)
	}
}

func TestDecideCellOwnTop(t *testing.T) {
	got := DecideCell(3, 5, true, true, 2, false, false, 0)
	want := CellSpec{Glyph: "🮎", FG: 2, BG: 5}
	if got != want {
		t.Errorf("own top: got %+v, want %+v", got, want)
	}
}

func TestDecideCellOwnBottom(t *testing.T) {
	got := DecideCell(3, 5, true, false, 2, false, false, 0)
	want := CellSpec{Glyph: "🮏", FG: 2, BG: 3}
	if got != want {
		t.Errorf("own bottom: got %+v, want %+v", got, want)
	}
}

func TestDecideCellOwnCursorWinsOverRemote(t *testing.T) {
	// Own cursor present along with a remote cursor: own color wins.
	got := DecideCell(3, 5, true, true, 2, true, true, 6)
	want := CellSpec{Glyph: "🮎", FG: 2, BG: 5}
	if got != want {
		t.Errorf("own wins: got %+v, want %+v", got, want)
	}
}

func TestDecideCellRemoteTop(t *testing.T) {
	got := DecideCell(3, 5, false, false, 0, true, true, 6)
	want := CellSpec{Glyph: "🮎", FG: 6, BG: 5}
	if got != want {
		t.Errorf("remote top: got %+v, want %+v", got, want)
	}
}

func TestDecideCellRemoteBottom(t *testing.T) {
	got := DecideCell(3, 5, false, false, 0, true, false, 6)
	want := CellSpec{Glyph: "🮏", FG: 6, BG: 3}
	if got != want {
		t.Errorf("remote bottom: got %+v, want %+v", got, want)
	}
}
