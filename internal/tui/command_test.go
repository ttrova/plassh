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
	if c := parseCommand("undo"); c.kind != cmdUndo || c.count != 1 || c.err != "" {
		t.Errorf("undo default: got %+v", c)
	}
	if c := parseCommand("fill 1 2 3 4"); c.kind != cmdFill || c.x != 1 || c.y != 2 || c.x2 != 3 || c.y2 != 4 {
		t.Errorf("fill: got %+v", c)
	}
	if c := parseCommand("line 0 0 9 9"); c.kind != cmdLine || c.x != 0 || c.y != 0 || c.x2 != 9 || c.y2 != 9 {
		t.Errorf("line: got %+v", c)
	}
	if c := parseCommand("clear"); c.kind != cmdClear || c.err != "" {
		t.Errorf("clear: got %+v", c)
	}
	if c := parseCommand("help"); c.kind != cmdHelp || c.err != "" {
		t.Errorf("help: got %+v", c)
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
		"fill 1 2 3",
		"line 1 2 3 x",
		"frobnicate 1",
	}
	for _, in := range bad {
		if c := parseCommand(in); c.err == "" {
			t.Errorf("parseCommand(%q) = %+v, want error", in, c)
		}
	}
}
