package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	return New(Deps{
		Width:  10,
		Height: 10,
		Grid:   make([]byte, 100),
		Name:   "tester",
		ID:     "id1",
	})
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestMoveRightWithL(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24 // window size set
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.cursorX != 1 {
		t.Errorf("cursorX = %d, want 1", m.cursorX)
	}
}

func TestMoveClampsAtLeftEdge(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	updated, _ := m.Update(keyMsg("h")) // already at x=0
	m = updated.(Model)
	if m.cursorX != 0 {
		t.Errorf("cursorX = %d, want 0", m.cursorX)
	}
}

func TestSelectColorByNumber(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(keyMsg("3"))
	m = updated.(Model)
	if m.selectedColor != 2 { // key "3" -> color index 2
		t.Errorf("selectedColor = %d, want 2", m.selectedColor)
	}
}

func TestTabCyclesColor(t *testing.T) {
	m := newTestModel()
	m.selectedColor = 7
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.selectedColor != 0 {
		t.Errorf("selectedColor = %d, want 0", m.selectedColor)
	}
}

func TestPaintUpdatesLocalGrid(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.selectedColor = 4
	m.cursorX, m.cursorY = 2, 3
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if m.grid[m.cursorY*m.canvasW+m.cursorX] != 4 {
		t.Errorf("painted pixel = %d, want 4", m.grid[m.cursorY*m.canvasW+m.cursorX])
	}
}

func TestDrawModeTrail(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.selectedColor = 3

	// 'd' enables draw mode and paints the starting pixel (0,0).
	updated, _ := m.Update(keyMsg("d"))
	m = updated.(Model)
	if !m.drawing {
		t.Fatal("expected draw mode on after 'd'")
	}
	if m.grid[0] != 3 {
		t.Errorf("start pixel (0,0) = %d, want 3", m.grid[0])
	}

	// Moving right while drawing paints the destination pixel (1,0).
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.cursorX != 1 {
		t.Fatalf("cursorX = %d, want 1", m.cursorX)
	}
	if m.grid[m.cursorY*m.canvasW+m.cursorX] != 3 {
		t.Errorf("trail pixel (1,0) not painted")
	}

	// 'd' again disables draw mode; further movement must NOT paint.
	updated, _ = m.Update(keyMsg("d"))
	m = updated.(Model)
	if m.drawing {
		t.Fatal("expected draw mode off after second 'd'")
	}
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.grid[m.cursorY*m.canvasW+m.cursorX] != 0 {
		t.Errorf("pixel (2,0) painted while draw mode off")
	}
}

func TestPixelUpdateMsgAppliesToGrid(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(PixelUpdateMsg{X: 1, Y: 1, Color: 6})
	m = updated.(Model)
	if m.grid[1*m.canvasW+1] != 6 {
		t.Errorf("grid not updated by PixelUpdateMsg")
	}
}

func TestPresenceUpdateAndGone(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(PresenceUpdateMsg{ID: "other", X: 5, Y: 5, Color: 2, Name: "x"})
	m = updated.(Model)
	if _, ok := m.remotes["other"]; !ok {
		t.Fatal("remote not tracked")
	}
	updated, _ = m.Update(PresenceGoneMsg{ID: "other"})
	m = updated.(Model)
	if _, ok := m.remotes["other"]; ok {
		t.Error("remote not removed on gone")
	}
}
