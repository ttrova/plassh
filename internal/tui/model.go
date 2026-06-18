// Package tui implements the per-session Bubble Tea program.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"plassh/internal/presence"
	"plassh/internal/render"
)

// Painter writes a pixel to shared storage (satisfied by *canvas.Canvas).
type Painter interface {
	SetPixel(ctx context.Context, x, y, color int) error
}

// Announcer publishes and refreshes this session's presence (satisfied by *presence.Presence).
type Announcer interface {
	Touch(ctx context.Context, u presence.Update) error
	Publish(ctx context.Context, u presence.Update) error
}

// Messages delivered into the program from external sources.
type (
	PixelUpdateMsg    struct{ X, Y, Color int }
	PresenceUpdateMsg struct {
		ID    string
		X, Y  int
		Color int
		Name  string
	}
	PresenceGoneMsg struct{ ID string }
	heartbeatMsg    struct{}
)

// remote is a tracked remote cursor with a last-seen timestamp for expiry.
type remote struct {
	x, y, color int
	name        string
	lastSeen    time.Time
}

// presenceEvent is the union the SSH layer feeds in for presence (Task 12).
type presenceEvent struct {
	Update presence.Update
}

// pixelChange records a pixel's color before and after a change, so the action
// can be undone (restore prev) and redone (re-apply next).
type pixelChange struct{ x, y, prev, next int }

// action is one undoable operation (a dab, a /circle, or a draw-mode stroke):
// the set of pixels it changed, with their previous colors.
type action []pixelChange

// write is a pending pixel write to flush to Redis.
type write struct{ x, y, c int }

// Deps are everything needed to build a Model. Channels and the Redis-backed
// dependencies are optional in tests (nil-safe).
type Deps struct {
	Ctx       context.Context
	Renderer  *lipgloss.Renderer
	Width     int
	Height    int
	Grid      []byte
	Name      string
	ID        string
	Painter   Painter
	Announcer Announcer
	Pixels    <-chan PixelUpdateMsg
	Presence  <-chan presenceEvent
	Disabled  map[string]bool // disabled slash-command names
}

// Model is the Bubble Tea model for one session.
type Model struct {
	ctx     context.Context
	canvasW int
	canvasH int
	grid    []byte

	id   string
	name string

	cursorX, cursorY int
	camX, camY       int
	selectedColor    int
	drawing          bool   // continuous draw mode: movement paints a trail
	stroke           action // pixels accumulated during the current draw stroke

	commandMode  bool   // typing a slash command
	commandInput string // command text being typed (without leading '/')
	statusMsg    string // transient feedback shown in the status bar
	showHelp     bool   // help overlay is up
	undoStack    []action
	redoStack    []action
	disabled     map[string]bool

	width, height int // terminal size in cells
	remotes       map[string]remote

	renderer  *lipgloss.Renderer
	styler    *render.CellStyler
	painter   Painter
	announcer Announcer
	pixels    <-chan PixelUpdateMsg
	presence  <-chan presenceEvent
}

const heartbeatInterval = 3 * time.Second

// New constructs a Model from its dependencies.
func New(d Deps) Model {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	renderer := d.Renderer
	if renderer == nil {
		renderer = lipgloss.DefaultRenderer()
	}
	disabled := d.Disabled
	if disabled == nil {
		disabled = make(map[string]bool)
	}
	return Model{
		ctx:           ctx,
		canvasW:       d.Width,
		canvasH:       d.Height,
		grid:          d.Grid,
		id:            d.ID,
		name:          d.Name,
		selectedColor: 1, // start on red (index 1); black-on-black is invisible
		remotes:       make(map[string]remote),
		renderer:      renderer,
		styler:        render.NewCellStyler(renderer),
		disabled:      disabled,
		painter:       d.Painter,
		announcer:     d.Announcer,
		pixels:        d.Pixels,
		presence:      d.Presence,
	}
}

// Init starts the message pumps and heartbeat, and announces this session's
// presence immediately so other users see the new cursor without waiting for the
// first heartbeat.
func (m Model) Init() tea.Cmd {
	return tea.Batch(waitPixel(m.pixels), waitPresence(m.presence), heartbeat(), m.announce())
}

// Update handles all messages and returns the next model + command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.recenter()
		return m, nil

	case tea.KeyMsg:
		if m.commandMode {
			return m.handleCommandKey(msg)
		}
		return m.handleKey(msg)

	case PixelUpdateMsg:
		if m.inBounds(msg.X, msg.Y) {
			m.grid[msg.Y*m.canvasW+msg.X] = byte(msg.Color)
		}
		return m, waitPixel(m.pixels)

	case PresenceUpdateMsg:
		if msg.ID != m.id {
			m.remotes[msg.ID] = remote{x: msg.X, y: msg.Y, color: msg.Color, name: msg.Name, lastSeen: time.Now()}
		}
		return m, waitPresence(m.presence)

	case PresenceGoneMsg:
		delete(m.remotes, msg.ID)
		return m, waitPresence(m.presence)

	case heartbeatMsg:
		m.expireStale()
		if m.announcer != nil {
			u := m.selfPresence()
			_ = m.announcer.Touch(m.ctx, u)
			_ = m.announcer.Publish(m.ctx, u)
		}
		return m, heartbeat()
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.statusMsg = "" // any normal key clears the last command's feedback
	m.showHelp = false
	moved := false
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyUp:
		m.cursorY, moved = clampMove(m.cursorY-1, m.canvasH)
	case tea.KeyDown:
		m.cursorY, moved = clampMove(m.cursorY+1, m.canvasH)
	case tea.KeyLeft:
		m.cursorX, moved = clampMove(m.cursorX-1, m.canvasW)
	case tea.KeyRight:
		m.cursorX, moved = clampMove(m.cursorX+1, m.canvasW)
	case tea.KeyTab:
		m.selectedColor = render.NextColor(m.selectedColor)
		cmd := m.announce()
		return *m, cmd
	case tea.KeySpace:
		cmd := m.dab()
		return *m, cmd
	case tea.KeyRunes:
		return m.handleRune(msg.Runes)
	}
	if moved {
		cmd := m.afterMove()
		return *m, cmd
	}
	return *m, nil
}

func (m *Model) handleRune(runes []rune) (tea.Model, tea.Cmd) {
	if len(runes) != 1 {
		return *m, nil
	}
	r := runes[0]
	moved := false
	switch r {
	case 'q':
		return *m, tea.Quit
	case '/':
		// Open the command line; keys now feed the input until Enter/Esc.
		m.commandMode = true
		m.commandInput = ""
		m.statusMsg = ""
		return *m, nil
	case 'd':
		// Toggle continuous draw mode. Turning it on begins a stroke and drops a
		// pixel at the cursor; turning it off commits the stroke as one undo action.
		m.drawing = !m.drawing
		if m.drawing {
			m.stroke = nil
			if w, ok := m.stage(m.cursorX, m.cursorY, m.selectedColor, &m.stroke); ok {
				return *m, m.flush([]write{w})
			}
			return *m, nil
		}
		m.pushUndo(m.stroke)
		m.stroke = nil
		return *m, nil
	case 'h':
		m.cursorX, moved = clampMove(m.cursorX-1, m.canvasW)
	case 'l':
		m.cursorX, moved = clampMove(m.cursorX+1, m.canvasW)
	case 'k':
		m.cursorY, moved = clampMove(m.cursorY-1, m.canvasH)
	case 'j':
		m.cursorY, moved = clampMove(m.cursorY+1, m.canvasH)
	case '1', '2', '3', '4', '5', '6', '7', '8':
		m.selectedColor = int(r - '1')
		cmd := m.announce()
		return *m, cmd
	}
	if moved {
		cmd := m.afterMove()
		return *m, cmd
	}
	return *m, nil
}

// afterMove recenters the camera, announces the new cursor position, and — when
// draw mode is active — paints the pixel the cursor just moved onto (into the
// current stroke), so moving lays down a continuous trail.
func (m *Model) afterMove() tea.Cmd {
	m.recenter()
	cmds := []tea.Cmd{m.announce()}
	if m.drawing {
		if w, ok := m.stage(m.cursorX, m.cursorY, m.selectedColor, &m.stroke); ok {
			cmds = append(cmds, m.flush([]write{w}))
		}
	}
	return tea.Batch(cmds...)
}

// dab paints a single pixel at the cursor as its own undo action.
func (m *Model) dab() tea.Cmd {
	var act action
	w, ok := m.stage(m.cursorX, m.cursorY, m.selectedColor, &act)
	if !ok {
		return nil
	}
	m.pushUndo(act)
	return m.flush([]write{w})
}

// stage records the pixel's previous color into act, applies the new color to
// the local grid, and returns the pending Redis write. ok is false (and nothing
// changes) when (x,y) is out of bounds.
func (m *Model) stage(x, y, color int, act *action) (write, bool) {
	if !m.inBounds(x, y) {
		return write{}, false
	}
	idx := y*m.canvasW + x
	*act = append(*act, pixelChange{x: x, y: y, prev: int(m.grid[idx]), next: color})
	m.grid[idx] = byte(color)
	return write{x: x, y: y, c: color}, true
}

// pushUndo records a completed action and clears the redo stack (a new action
// invalidates the redo history).
func (m *Model) pushUndo(act action) {
	if len(act) == 0 {
		return
	}
	m.undoStack = append(m.undoStack, act)
	m.redoStack = nil
}

// flush returns a single command that writes all pending pixels to Redis in one
// goroutine, so a large /circle or /undo is one command rather than thousands.
func (m *Model) flush(ws []write) tea.Cmd {
	if m.painter == nil || len(ws) == 0 {
		return nil
	}
	painter, ctx := m.painter, m.ctx
	return func() tea.Msg {
		for _, w := range ws {
			_ = painter.SetPixel(ctx, w.x, w.y, w.c)
		}
		return nil
	}
}

// announce publishes this session's cursor/color to others.
func (m *Model) announce() tea.Cmd {
	if m.announcer == nil {
		return nil
	}
	u := m.selfPresence()
	return func() tea.Msg {
		_ = m.announcer.Touch(m.ctx, u)
		_ = m.announcer.Publish(m.ctx, u)
		return nil
	}
}

func (m Model) selfPresence() presence.Update {
	return presence.Update{ID: m.id, X: m.cursorX, Y: m.cursorY, Color: m.selectedColor, Name: m.name}
}

// handleCommandKey feeds keystrokes into the command line while command mode is
// active: Enter runs it, Esc cancels, Backspace edits, other keys type.
func (m *Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyEsc:
		m.commandMode = false
		m.commandInput = ""
		return *m, nil
	case tea.KeyEnter:
		cmd := m.runCommand(parseCommand(m.commandInput))
		m.commandMode = false
		m.commandInput = ""
		return *m, cmd
	case tea.KeyBackspace:
		if r := []rune(m.commandInput); len(r) > 0 {
			m.commandInput = string(r[:len(r)-1])
		}
		return *m, nil
	case tea.KeySpace:
		m.commandInput += " "
		return *m, nil
	case tea.KeyRunes:
		m.commandInput += string(msg.Runes)
		return *m, nil
	}
	return *m, nil
}

// maxCircleRadius and maxUndo bound the two unbounded commands.
const (
	maxCircleRadius = 10
	maxUndo         = 10
)

// runCommand executes a parsed command, sets a status message, and returns any
// Redis writes / presence updates it produced.
func (m *Model) runCommand(c command) tea.Cmd {
	if c.kind != cmdUnknown && m.disabled[c.name] {
		m.statusMsg = "/" + c.name + " is disabled"
		return nil
	}
	switch c.kind {
	case cmdTP:
		m.cursorX = clampInt(c.x, 0, m.canvasW-1)
		m.cursorY = clampInt(c.y, 0, m.canvasH-1)
		m.recenter()
		m.statusMsg = fmt.Sprintf("teleported to %d,%d", m.cursorX, m.cursorY)
		return m.announce()
	case cmdCircle:
		return m.drawCircle(c.size)
	case cmdFill:
		return m.fillRect(c.x, c.y, c.x2, c.y2)
	case cmdLine:
		return m.drawLine(c.x, c.y, c.x2, c.y2)
	case cmdFlood:
		return m.flood()
	case cmdClear:
		return m.clear()
	case cmdUndo:
		return m.undo(c.count)
	case cmdRedo:
		return m.redo(c.count)
	case cmdHelp:
		m.showHelp = true
		return nil
	default:
		m.statusMsg = c.err
		return nil
	}
}

// commitPaint records a completed paint action and returns the flush command.
func (m *Model) commitPaint(act action, ws []write, msg string) tea.Cmd {
	m.pushUndo(act)
	m.statusMsg = msg
	return m.flush(ws)
}

// drawCircle paints a filled disk of the given radius centered on the cursor in
// the current color, recorded as a single undo action. Radius is capped.
func (m *Model) drawCircle(size int) tea.Cmd {
	if size > maxCircleRadius {
		size = maxCircleRadius
	}
	cx, cy := m.cursorX, m.cursorY
	r2 := size * size
	var act action
	var ws []write
	for dy := -size; dy <= size; dy++ {
		for dx := -size; dx <= size; dx++ {
			if dx*dx+dy*dy > r2 {
				continue
			}
			if w, ok := m.stage(cx+dx, cy+dy, m.selectedColor, &act); ok {
				ws = append(ws, w)
			}
		}
	}
	return m.commitPaint(act, ws, fmt.Sprintf("circle r=%d (%d px)", size, len(ws)))
}

// fillRect paints the rectangle spanned by the two corners in the current color,
// as one undo action.
func (m *Model) fillRect(x1, y1, x2, y2 int) tea.Cmd {
	x1, x2 = clampInt(min(x1, x2), 0, m.canvasW-1), clampInt(max(x1, x2), 0, m.canvasW-1)
	y1, y2 = clampInt(min(y1, y2), 0, m.canvasH-1), clampInt(max(y1, y2), 0, m.canvasH-1)
	var act action
	var ws []write
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			if w, ok := m.stage(x, y, m.selectedColor, &act); ok {
				ws = append(ws, w)
			}
		}
	}
	return m.commitPaint(act, ws, fmt.Sprintf("filled %d px", len(ws)))
}

// drawLine paints a Bresenham line between the two points in the current color,
// as one undo action.
func (m *Model) drawLine(x1, y1, x2, y2 int) tea.Cmd {
	x1, y1 = clampInt(x1, 0, m.canvasW-1), clampInt(y1, 0, m.canvasH-1)
	x2, y2 = clampInt(x2, 0, m.canvasW-1), clampInt(y2, 0, m.canvasH-1)
	dx, dy := abs(x2-x1), -abs(y2-y1)
	sx, sy := sign(x2-x1), sign(y2-y1)
	err := dx + dy
	var act action
	var ws []write
	for {
		if w, ok := m.stage(x1, y1, m.selectedColor, &act); ok {
			ws = append(ws, w)
		}
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x1 += sx
		}
		if e2 <= dx {
			err += dx
			y1 += sy
		}
	}
	return m.commitPaint(act, ws, fmt.Sprintf("line (%d px)", len(ws)))
}

// clear resets every painted pixel to black, as one undo action.
func (m *Model) clear() tea.Cmd {
	var act action
	var ws []write
	for y := 0; y < m.canvasH; y++ {
		for x := 0; x < m.canvasW; x++ {
			if m.grid[y*m.canvasW+x] == 0 {
				continue
			}
			if w, ok := m.stage(x, y, 0, &act); ok {
				ws = append(ws, w)
			}
		}
	}
	return m.commitPaint(act, ws, fmt.Sprintf("cleared %d px", len(ws)))
}

// flood fills the contiguous region of same-colored pixels connected to the
// cursor with the current color, as one undo action (4-directional).
func (m *Model) flood() tea.Cmd {
	if !m.inBounds(m.cursorX, m.cursorY) {
		return nil
	}
	target := int(m.grid[m.cursorY*m.canvasW+m.cursorX])
	if target == m.selectedColor {
		m.statusMsg = "nothing to fill"
		return nil
	}
	var act action
	var ws []write
	queue := [][2]int{{m.cursorX, m.cursorY}}
	for len(queue) > 0 {
		p := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x, y := p[0], p[1]
		if !m.inBounds(x, y) || int(m.grid[y*m.canvasW+x]) != target {
			continue
		}
		w, _ := m.stage(x, y, m.selectedColor, &act) // sets pixel, marks visited
		ws = append(ws, w)
		queue = append(queue, [2]int{x + 1, y}, [2]int{x - 1, y}, [2]int{x, y + 1}, [2]int{x, y - 1})
	}
	return m.commitPaint(act, ws, fmt.Sprintf("flood %d px", len(ws)))
}

// undo reverts up to n of this session's most recent actions, restoring each
// affected pixel to its previous color, and moves them onto the redo stack.
func (m *Model) undo(n int) tea.Cmd {
	if n > maxUndo {
		n = maxUndo
	}
	if len(m.undoStack) == 0 {
		m.statusMsg = "nothing to undo"
		return nil
	}
	var ws []write
	undone := 0
	for n > 0 && len(m.undoStack) > 0 {
		act := m.undoStack[len(m.undoStack)-1]
		m.undoStack = m.undoStack[:len(m.undoStack)-1]
		for i := len(act) - 1; i >= 0; i-- {
			pc := act[i]
			m.grid[pc.y*m.canvasW+pc.x] = byte(pc.prev)
			ws = append(ws, write{x: pc.x, y: pc.y, c: pc.prev})
		}
		m.redoStack = append(m.redoStack, act)
		undone++
		n--
	}
	m.statusMsg = fmt.Sprintf("undid %d action(s)", undone)
	return m.flush(ws)
}

// redo re-applies up to n previously-undone actions, moving them back onto the
// undo stack.
func (m *Model) redo(n int) tea.Cmd {
	if n > maxUndo {
		n = maxUndo
	}
	if len(m.redoStack) == 0 {
		m.statusMsg = "nothing to redo"
		return nil
	}
	var ws []write
	redone := 0
	for n > 0 && len(m.redoStack) > 0 {
		act := m.redoStack[len(m.redoStack)-1]
		m.redoStack = m.redoStack[:len(m.redoStack)-1]
		for _, pc := range act {
			m.grid[pc.y*m.canvasW+pc.x] = byte(pc.next)
			ws = append(ws, write{x: pc.x, y: pc.y, c: pc.next})
		}
		m.undoStack = append(m.undoStack, act) // raw append: must not clear redo here
		redone++
		n--
	}
	m.statusMsg = fmt.Sprintf("redid %d action(s)", redone)
	return m.flush(ws)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

// viewportPixels is the visible pixel area, never larger than the canvas itself.
// When the terminal is bigger than the canvas the extra space stays outside the
// border, so the border sits on the real canvas edge rather than the window edge.
func (m Model) viewportPixels() (pw, ph int) {
	pw = render.VisiblePixelWidth(m.width)
	ph = render.VisiblePixelHeight(m.height)
	if pw > m.canvasW {
		pw = m.canvasW
	}
	evenH := m.canvasH + (m.canvasH & 1) // half-block cells are 2px tall
	if ph > evenH {
		ph = evenH
	}
	return pw, ph
}

func (m *Model) recenter() {
	pw, ph := m.viewportPixels()
	if pw <= 0 || ph <= 0 {
		return
	}
	m.camX = render.CameraFor(m.cursorX, m.camX, pw, m.canvasW)
	m.camY = render.CameraFor(m.cursorY, m.camY, ph, m.canvasH)
}

func (m *Model) expireStale() {
	cutoff := time.Now().Add(-presence.TTL)
	for id, r := range m.remotes {
		if r.lastSeen.Before(cutoff) {
			delete(m.remotes, id)
		}
	}
}

func (m Model) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.canvasW && y < m.canvasH
}

// clampMove returns the clamped coordinate and whether it actually moved to a
// different in-bounds cell. Out-of-range input clamps to the edge and reports
// no movement.
func clampMove(v, size int) (int, bool) {
	if v < 0 {
		return 0, false
	}
	if v >= size {
		return size - 1, false
	}
	return v, true
}

// View renders the players header, bordered canvas, and status bar (or the help
// overlay when it is up).
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "initializing..."
	}
	if m.showHelp {
		return m.helpView()
	}
	pw, ph := m.viewportPixels()

	canvas := render.Canvas(render.View{
		Styler: m.styler,
		Grid:   m.grid, Width: m.canvasW, Height: m.canvasH,
		CamX: m.camX, CamY: m.camY, PixelCols: pw, PixelRows: ph,
		CursorX: m.cursorX, CursorY: m.cursorY, SelectedColor: m.selectedColor,
		Remotes: m.remoteCursors(),
	})

	// Highlight the sides where the true canvas edge is currently in view, so the
	// user can tell a real boundary from a mid-canvas scroll edge.
	edgeColor := lipgloss.Color("11")  // bright yellow = real canvas edge
	scrollColor := lipgloss.Color("8") // dim grey = more canvas beyond
	sideColor := func(atEdge bool) lipgloss.Color {
		if atEdge {
			return edgeColor
		}
		return scrollColor
	}
	style := m.renderer.NewStyle().Border(lipgloss.NormalBorder()).
		BorderTopForeground(sideColor(m.camY == 0)).
		BorderBottomForeground(sideColor(m.camY+ph >= m.canvasH)).
		BorderLeftForeground(sideColor(m.camX == 0)).
		BorderRightForeground(sideColor(m.camX+pw >= m.canvasW))

	// Clip the header and status bar to the terminal width so they never wrap onto
	// extra lines (which would push the layout past the screen on small terminals).
	clip := m.renderer.NewStyle().MaxWidth(m.width)
	header := clip.Render(m.playerList())
	status := clip.Render(m.statusBar())
	return header + "\n" + style.Render(canvas) + "\n" + status
}

// playerList renders the connected players (you first) with color swatches.
func (m Model) playerList() string {
	swatch := func(c int) string {
		return m.renderer.NewStyle().Foreground(render.ColorAt(c)).Render("█")
	}
	parts := []string{swatch(m.selectedColor) + m.name + " (you)"}
	for _, r := range m.remotes {
		parts = append(parts, swatch(r.color)+r.name)
	}
	return fmt.Sprintf("Players (%d): %s", len(m.remotes)+1, strings.Join(parts, "  "))
}

// helpView renders the command help as a bordered panel.
func (m Model) helpView() string {
	type entry struct{ usage, desc string }
	entries := []entry{
		{"/tp x y", "move the cursor"},
		{"/circle <r>", "filled disk, radius r (max 10)"},
		{"/fill x1 y1 x2 y2", "fill a rectangle"},
		{"/line x1 y1 x2 y2", "draw a line"},
		{"/flood", "flood-fill the region under the cursor"},
		{"/clear", "clear the whole canvas"},
		{"/undo [n]", "undo your last n actions (default 1, max 10)"},
		{"/redo [n]", "redo undone actions (default 1, max 10)"},
		{"/help", "this help"},
	}
	var b strings.Builder
	b.WriteString("Commands\n\n")
	for _, e := range entries {
		name := strings.TrimPrefix(strings.Fields(e.usage)[0], "/")
		line := fmt.Sprintf("  %-20s %s", e.usage, e.desc)
		if m.disabled[name] {
			line += "  (disabled)"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n(press any key to dismiss)")
	panel := m.renderer.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Render(b.String())
	return m.renderer.NewStyle().MaxWidth(m.width).Render(panel)
}

func (m Model) remoteCursors() []render.RemoteCursor {
	out := make([]render.RemoteCursor, 0, len(m.remotes))
	for _, r := range m.remotes {
		out = append(out, render.RemoteCursor{X: r.x, Y: r.y, Color: r.color})
	}
	return out
}

func (m Model) statusBar() string {
	if m.commandMode {
		return m.renderer.NewStyle().Bold(true).Render("/" + m.commandInput + "█")
	}
	swatch := m.renderer.NewStyle().Foreground(render.ColorAt(m.selectedColor)).Render("█")
	draw := ""
	if m.drawing {
		draw = " │ " + m.renderer.NewStyle().Foreground(render.ColorAt(2)).Bold(true).Render("DRAW")
	}
	tail := "1-8/Tab color · Space dab · d draw · / cmd · q quit"
	if m.statusMsg != "" {
		tail = m.statusMsg
	}
	return fmt.Sprintf(
		"You: %s │ Color: %s %s (%d/8) │ Pos: %d,%d │ Users: %d%s │ %s",
		m.name, swatch, render.ColorName(m.selectedColor), m.selectedColor+1,
		m.cursorX, m.cursorY, len(m.remotes)+1, draw,
		tail,
	)
}

func waitPixel(ch <-chan PixelUpdateMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return nil
		}
		return u
	}
}

func waitPresence(ch <-chan presenceEvent) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
		if e.Update.Gone {
			return PresenceGoneMsg{ID: e.Update.ID}
		}
		return PresenceUpdateMsg{ID: e.Update.ID, X: e.Update.X, Y: e.Update.Y, Color: e.Update.Color, Name: e.Update.Name}
	}
}

func heartbeat() tea.Cmd {
	return tea.Tick(heartbeatInterval, func(time.Time) tea.Msg { return heartbeatMsg{} })
}
