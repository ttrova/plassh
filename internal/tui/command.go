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
)

// command is the result of parsing a command-line entry (without the leading
// '/'). A non-empty err means the input was invalid and should be shown to the
// user instead of executed.
type command struct {
	kind  commandKind
	x, y  int    // cmdTP
	size  int    // cmdCircle (radius)
	count int    // cmdUndo
	err   string // usage / parse error
}

// parseCommand parses a command line such as "tp 10 20", "circle 5" or
// "undo 3". The leading '/' is not included.
func parseCommand(input string) command {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return command{err: "empty command"}
	}
	name, args := strings.ToLower(fields[0]), fields[1:]
	switch name {
	case "tp":
		if len(args) != 2 {
			return command{err: "usage: /tp x y"}
		}
		x, errX := strconv.Atoi(args[0])
		y, errY := strconv.Atoi(args[1])
		if errX != nil || errY != nil {
			return command{err: "tp: x and y must be integers"}
		}
		return command{kind: cmdTP, x: x, y: y}
	case "circle":
		if len(args) != 1 {
			return command{err: "usage: /circle <radius>"}
		}
		s, err := strconv.Atoi(args[0])
		if err != nil || s < 0 {
			return command{err: "circle: radius must be a non-negative integer"}
		}
		return command{kind: cmdCircle, size: s}
	case "undo":
		if len(args) != 1 {
			return command{err: "usage: /undo <count>"}
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return command{err: "undo: count must be a positive integer"}
		}
		return command{kind: cmdUndo, count: n}
	default:
		return command{err: fmt.Sprintf("unknown command: %s", name)}
	}
}
