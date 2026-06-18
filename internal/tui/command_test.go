package tui

import "testing"

func TestParseCommandValid(t *testing.T) {
	if c := parseCommand("tp 10 20"); c.kind != cmdTP || c.x != 10 || c.y != 20 || c.err != "" {
		t.Errorf("tp: got %+v", c)
	}
	if c := parseCommand("circle 5"); c.kind != cmdCircle || c.size != 5 || c.err != "" {
		t.Errorf("circle: got %+v", c)
	}
	if c := parseCommand("undo 3"); c.kind != cmdUndo || c.count != 3 || c.err != "" {
		t.Errorf("undo: got %+v", c)
	}
	if c := parseCommand("  TP   1   2 "); c.kind != cmdTP || c.x != 1 || c.y != 2 {
		t.Errorf("case/spacing: got %+v", c)
	}
}

func TestParseCommandInvalid(t *testing.T) {
	bad := []string{
		"",
		"tp 1",
		"tp a b",
		"circle",
		"circle -1",
		"circle x",
		"undo 0",
		"undo -2",
		"undo two",
		"frobnicate 1",
	}
	for _, in := range bad {
		if c := parseCommand(in); c.err == "" {
			t.Errorf("parseCommand(%q) = %+v, want error", in, c)
		}
	}
}
