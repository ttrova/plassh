package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// commandKind identifies a parsed slash command.
type commandKind int

const (
	cmdUnknown commandKind = iota
	cmdTP
	cmdCircle
	cmdUndo
	cmdFill
	cmdLine
	cmdClear
	cmdHelp
)

// command is the result of parsing a command-line entry (without the leading
// '/'). A non-empty err means the input was invalid and should be shown to the
// user instead of executed.
type command struct {
	kind         commandKind
	name         string // canonical command name (for disable checks / messages)
	x, y, x2, y2 int    // tp/fill/line coordinates
	size         int    // circle radius
	count        int    // undo count
	err          string // usage / parse error
}

// parseCommand parses a command line such as "tp 10 20", "circle 5",
// "undo", "fill 1 1 5 5", "line 0 0 9 9", "clear" or "help". The leading '/' is
// not included.
func parseCommand(input string) command {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return command{err: "empty command"}
	}
	name, args := strings.ToLower(fields[0]), fields[1:]
	switch name {
	case "tp":
		xs, err := ints(args, 2)
		if err != "" {
			return command{err: "usage: /tp x y"}
		}
		return command{kind: cmdTP, name: name, x: xs[0], y: xs[1]}
	case "circle":
		xs, err := ints(args, 1)
		if err != "" || xs[0] < 0 {
			return command{err: "usage: /circle <radius>"}
		}
		return command{kind: cmdCircle, name: name, size: xs[0]}
	case "undo":
		if len(args) == 0 {
			return command{kind: cmdUndo, name: name, count: 1} // default /undo 1
		}
		xs, err := ints(args, 1)
		if err != "" || xs[0] < 1 {
			return command{err: "usage: /undo [count]"}
		}
		return command{kind: cmdUndo, name: name, count: xs[0]}
	case "fill":
		xs, err := ints(args, 4)
		if err != "" {
			return command{err: "usage: /fill x1 y1 x2 y2"}
		}
		return command{kind: cmdFill, name: name, x: xs[0], y: xs[1], x2: xs[2], y2: xs[3]}
	case "line":
		xs, err := ints(args, 4)
		if err != "" {
			return command{err: "usage: /line x1 y1 x2 y2"}
		}
		return command{kind: cmdLine, name: name, x: xs[0], y: xs[1], x2: xs[2], y2: xs[3]}
	case "clear":
		return command{kind: cmdClear, name: name}
	case "help":
		return command{kind: cmdHelp, name: name}
	default:
		return command{err: fmt.Sprintf("unknown command: %s", name)}
	}
}

// ints parses exactly n integer arguments, returning a non-empty error string on
// the wrong count or a non-numeric value.
func ints(args []string, n int) ([]int, string) {
	if len(args) != n {
		return nil, fmt.Sprintf("expected %d arguments", n)
	}
	out := make([]int, n)
	for i, a := range args {
		v, err := strconv.Atoi(a)
		if err != nil {
			return nil, fmt.Sprintf("argument %d is not an integer", i+1)
		}
		out[i] = v
	}
	return out, ""
}
