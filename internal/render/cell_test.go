package render

import "testing"

var (
	g0top = string(rune(0x1FB8E)) // 🮎 style 0 top
	g0bot = string(rune(0x1FB8F)) // 🮏 style 0 bottom
	g1top = string(rune(0x1FB82)) // 🮂 style 1 top
)

func TestDecideCellNormal(t *testing.T) {
	got := DecideCell(3, 5, Cursor{}, Cursor{})
	want := CellSpec{Glyph: "▀", FG: 3, BG: 5}
	if got != want {
		t.Errorf("normal: got %+v, want %+v", got, want)
	}
}

func TestDecideCellOwnTop(t *testing.T) {
	got := DecideCell(3, 5, Cursor{Here: true, OnTop: true, Color: 2, Style: 0}, Cursor{})
	want := CellSpec{Glyph: g0top, FG: 2, BG: 5}
	if got != want {
		t.Errorf("own top: got %+v, want %+v", got, want)
	}
}

func TestDecideCellOwnBottom(t *testing.T) {
	got := DecideCell(3, 5, Cursor{Here: true, OnTop: false, Color: 2, Style: 0}, Cursor{})
	want := CellSpec{Glyph: g0bot, FG: 2, BG: 3}
	if got != want {
		t.Errorf("own bottom: got %+v, want %+v", got, want)
	}
}

func TestDecideCellStyleVaries(t *testing.T) {
	got := DecideCell(3, 5, Cursor{Here: true, OnTop: true, Color: 4, Style: 1}, Cursor{})
	if got.Glyph != g1top {
		t.Errorf("style 1 top glyph = %q, want %q", got.Glyph, g1top)
	}
}

func TestDecideCellOwnWinsOverRemote(t *testing.T) {
	got := DecideCell(3, 5,
		Cursor{Here: true, OnTop: true, Color: 2, Style: 0},
		Cursor{Here: true, OnTop: true, Color: 6, Style: 1})
	want := CellSpec{Glyph: g0top, FG: 2, BG: 5}
	if got != want {
		t.Errorf("own wins: got %+v, want %+v", got, want)
	}
}

func TestDecideCellRemote(t *testing.T) {
	if got := DecideCell(3, 5, Cursor{}, Cursor{Here: true, OnTop: true, Color: 6, Style: 0}); got != (CellSpec{Glyph: g0top, FG: 6, BG: 5}) {
		t.Errorf("remote top: got %+v", got)
	}
	if got := DecideCell(3, 5, Cursor{}, Cursor{Here: true, OnTop: false, Color: 6, Style: 0}); got != (CellSpec{Glyph: g0bot, FG: 6, BG: 3}) {
		t.Errorf("remote bottom: got %+v", got)
	}
}

func TestStyleForDeterministic(t *testing.T) {
	if StyleFor("abc") != StyleFor("abc") {
		t.Error("StyleFor must be deterministic")
	}
	if s := StyleFor("abc"); s < 0 || s >= len(cursorStyles) {
		t.Errorf("StyleFor out of range: %d", s)
	}
}
