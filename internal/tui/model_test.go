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

func send(m Model, msg tea.Msg) Model {
	u, _ := m.Update(msg)
	return u.(Model)
}

func TestCommandModeTeleport(t *testing.T) {
	m := newTestModel() // canvas 10x10
	m.width, m.height = 80, 24

	m = send(m, keyMsg("/"))
	if !m.commandMode {
		t.Fatal("expected command mode after '/'")
	}
	for _, ch := range []string{"t", "p"} {
		m = send(m, keyMsg(ch))
	}
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	m = send(m, keyMsg("3"))
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	m = send(m, keyMsg("4"))
	if m.commandInput != "tp 3 4" {
		t.Fatalf("commandInput = %q, want %q", m.commandInput, "tp 3 4")
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.commandMode {
		t.Error("expected command mode to exit on Enter")
	}
	if m.cursorX != 3 || m.cursorY != 4 {
		t.Errorf("cursor = %d,%d, want 3,4", m.cursorX, m.cursorY)
	}
}

func TestCommandTeleportClamps(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.runCommand(parseCommand("tp 999 999"))
	if m.cursorX != 9 || m.cursorY != 9 {
		t.Errorf("cursor = %d,%d, want 9,9 (clamped)", m.cursorX, m.cursorY)
	}
}

func TestCommandCircleAndUndo(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.selectedColor = 2
	m.cursorX, m.cursorY = 5, 5

	m.runCommand(parseCommand("circle 1")) // filled disk radius 1 = plus of 5 pixels
	plus := [][2]int{{5, 5}, {6, 5}, {4, 5}, {5, 6}, {5, 4}}
	for _, p := range plus {
		if got := m.grid[p[1]*m.canvasW+p[0]]; got != 2 {
			t.Errorf("pixel %v = %d, want 2", p, got)
		}
	}
	if len(m.undoStack) != 1 {
		t.Fatalf("undoStack len = %d, want 1", len(m.undoStack))
	}

	m.runCommand(parseCommand("undo 1"))
	for _, p := range plus {
		if got := m.grid[p[1]*m.canvasW+p[0]]; got != 0 {
			t.Errorf("after undo pixel %v = %d, want 0", p, got)
		}
	}
	if len(m.undoStack) != 0 {
		t.Errorf("undoStack len = %d, want 0 after undo", len(m.undoStack))
	}
}

func TestDrawStrokeIsOneUndoAction(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24 // default color 1

	m = send(m, keyMsg("d")) // pen down at (0,0)
	m = send(m, keyMsg("l")) // (1,0)
	m = send(m, keyMsg("l")) // (2,0)
	m = send(m, keyMsg("d")) // pen up -> commit one action

	if len(m.undoStack) != 1 {
		t.Fatalf("undoStack len = %d, want 1 (whole stroke)", len(m.undoStack))
	}
	for x := 0; x <= 2; x++ {
		if m.grid[x] != 1 {
			t.Errorf("stroke pixel (%d,0) = %d, want 1", x, m.grid[x])
		}
	}
	m.runCommand(parseCommand("undo 1"))
	for x := 0; x <= 2; x++ {
		if m.grid[x] != 0 {
			t.Errorf("after undo (%d,0) = %d, want 0", x, m.grid[x])
		}
	}
}

func TestCommandFillRect(t *testing.T) {
	m := newTestModel() // 10x10
	m.selectedColor = 5
	m.runCommand(parseCommand("fill 1 1 3 2")) // x:1..3, y:1..2 => 6 pixels
	for y := 1; y <= 2; y++ {
		for x := 1; x <= 3; x++ {
			if m.grid[y*m.canvasW+x] != 5 {
				t.Errorf("fill pixel %d,%d = %d, want 5", x, y, m.grid[y*m.canvasW+x])
			}
		}
	}
	if m.grid[0] != 0 {
		t.Error("pixel outside rect should be untouched")
	}
	if len(m.undoStack) != 1 {
		t.Errorf("fill should be one action, got %d", len(m.undoStack))
	}
}

func TestCommandLine(t *testing.T) {
	m := newTestModel()
	m.selectedColor = 4
	m.runCommand(parseCommand("line 0 0 3 0")) // horizontal line y=0, x:0..3
	for x := 0; x <= 3; x++ {
		if m.grid[x] != 4 {
			t.Errorf("line pixel %d = %d, want 4", x, m.grid[x])
		}
	}
	if len(m.undoStack) != 1 {
		t.Errorf("line should be one action, got %d", len(m.undoStack))
	}
}

func TestCommandClear(t *testing.T) {
	m := newTestModel()
	m.grid[0] = 3
	m.grid[55] = 7
	m.runCommand(parseCommand("clear"))
	for i, b := range m.grid {
		if b != 0 {
			t.Fatalf("grid[%d] = %d after clear, want 0", i, b)
		}
	}
	// clear is undoable
	m.runCommand(parseCommand("undo"))
	if m.grid[0] != 3 || m.grid[55] != 7 {
		t.Error("undo should restore cleared pixels")
	}
}

func TestCircleRadiusCapped(t *testing.T) {
	m := newTestModel()
	m.cursorX, m.cursorY = 5, 5
	m.runCommand(parseCommand("circle 999"))
	// Radius capped at 10; with a 10x10 canvas it just fills what's in bounds,
	// but it must not panic or run unbounded. Status reports the capped radius.
	if want := "circle r=10"; len(m.statusMsg) < len(want) || m.statusMsg[:len(want)] != want {
		t.Errorf("statusMsg = %q, want prefix %q", m.statusMsg, want)
	}
}

func TestUndoCountCapped(t *testing.T) {
	m := newTestModel()
	// push 15 dab actions
	for i := 0; i < 15; i++ {
		m.cursorX = i % m.canvasW
		m.dab()
	}
	m.runCommand(parseCommand("undo 999")) // capped to 10
	if len(m.undoStack) != 5 {
		t.Errorf("undoStack = %d after capped undo, want 5", len(m.undoStack))
	}
}

func TestDisabledCommand(t *testing.T) {
	m := New(Deps{Width: 10, Height: 10, Grid: make([]byte, 100), Disabled: map[string]bool{"clear": true}})
	m.grid[0] = 3
	m.runCommand(parseCommand("clear"))
	if m.grid[0] != 3 {
		t.Error("disabled /clear should not run")
	}
	if m.statusMsg == "" {
		t.Error("expected a disabled message")
	}
}

func TestHelpToggles(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.runCommand(parseCommand("help"))
	if !m.showHelp {
		t.Fatal("expected showHelp true")
	}
	m = send(m, keyMsg("x")) // any normal key dismisses
	if m.showHelp {
		t.Error("expected help dismissed on keypress")
	}
}

func TestCommandFlood(t *testing.T) {
	m := newTestModel() // 10x10, all 0
	m.selectedColor = 6
	m.cursorX, m.cursorY = 5, 5
	// Put a wall so the flood is bounded to a region.
	for y := 0; y < 10; y++ {
		m.grid[y*m.canvasW+3] = 1 // vertical wall at x=3 (color 1)
	}
	m.runCommand(parseCommand("flood"))
	// Region right of the wall (x 4..9) should be color 6; left of wall stays 0.
	if m.grid[5*m.canvasW+4] != 6 || m.grid[5*m.canvasW+9] != 6 {
		t.Error("flood did not fill the region right of the wall")
	}
	if m.grid[5*m.canvasW+0] != 0 {
		t.Error("flood leaked across the wall")
	}
	if m.grid[5*m.canvasW+3] != 1 {
		t.Error("flood overwrote the wall")
	}
}

func TestUndoRedo(t *testing.T) {
	m := newTestModel()
	m.selectedColor = 7
	m.cursorX, m.cursorY = 2, 2
	m.dab() // paint (2,2)=7
	idx := 2*m.canvasW + 2
	if m.grid[idx] != 7 {
		t.Fatal("dab failed")
	}
	m.runCommand(parseCommand("undo"))
	if m.grid[idx] != 0 {
		t.Error("undo did not revert")
	}
	m.runCommand(parseCommand("redo"))
	if m.grid[idx] != 7 {
		t.Error("redo did not re-apply")
	}
}

func TestNewActionClearsRedo(t *testing.T) {
	m := newTestModel()
	m.selectedColor = 5
	m.cursorX, m.cursorY = 1, 1
	m.dab()
	m.runCommand(parseCommand("undo")) // redo stack now has 1
	m.cursorX = 4
	m.dab()                           // new action -> redo stack cleared
	if cmd := m.redo(1); cmd != nil { // redo should be a no-op now
		t.Error("new action should have cleared the redo stack")
	}
}

func TestViewportClampedToCanvas(t *testing.T) {
	m := newTestModel() // canvas 10x10

	// Terminal much larger than the canvas -> viewport capped at canvas size.
	m.width, m.height = 200, 100
	if pw, ph := m.viewportPixels(); pw != 10 || ph != 10 {
		t.Errorf("large terminal: got %dx%d pixels, want 10x10", pw, ph)
	}

	// Terminal smaller than the canvas -> viewport follows the window.
	// width 8 -> 6 cols; height 8 -> (8-4)*2 = 8 px rows.
	m.width, m.height = 8, 8
	if pw, ph := m.viewportPixels(); pw != 6 || ph != 8 {
		t.Errorf("small terminal: got %dx%d pixels, want 6x8", pw, ph)
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
