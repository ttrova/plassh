// Package tui implements the per-session Bubble Tea program.
package tui

import (
	"context"
	"fmt"
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

// pixelChange records a single pixel's prior color so an action can be undone.
type pixelChange struct{ x, y, prev int }

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
	undoStack    []action

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
		if len(m.stroke) > 0 {
			m.undoStack = append(m.undoStack, m.stroke)
		}
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
	m.undoStack = append(m.undoStack, act)
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
	*act = append(*act, pixelChange{x: x, y: y, prev: int(m.grid[idx])})
	m.grid[idx] = byte(color)
	return write{x: x, y: y, c: color}, true
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

// runCommand executes a parsed command, sets a status message, and returns any
// Redis writes / presence updates it produced.
func (m *Model) runCommand(c command) tea.Cmd {
	switch c.kind {
	case cmdTP:
		m.cursorX = clampInt(c.x, 0, m.canvasW-1)
		m.cursorY = clampInt(c.y, 0, m.canvasH-1)
		m.recenter()
		m.statusMsg = fmt.Sprintf("teleported to %d,%d", m.cursorX, m.cursorY)
		return m.announce()
	case cmdCircle:
		return m.drawCircle(c.size)
	case cmdUndo:
		return m.undo(c.count)
	default:
		m.statusMsg = c.err
		return nil
	}
}

// drawCircle paints a filled disk of the given radius centered on the cursor in
// the current color, recorded as a single undo action. The radius is capped to
// the canvas size to bound the work.
func (m *Model) drawCircle(size int) tea.Cmd {
	maxDim := m.canvasW
	if m.canvasH > maxDim {
		maxDim = m.canvasH
	}
	if size > maxDim {
		size = maxDim
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
	if len(act) > 0 {
		m.undoStack = append(m.undoStack, act)
	}
	m.statusMsg = fmt.Sprintf("circle r=%d (%d px)", size, len(ws))
	return m.flush(ws)
}

// undo reverts up to n of this session's most recent actions, restoring each
// affected pixel to the color it had before the action.
func (m *Model) undo(n int) tea.Cmd {
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
		undone++
		n--
	}
	m.statusMsg = fmt.Sprintf("undid %d action(s)", undone)
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

// View renders border + canvas + status bar.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "initializing..."
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

	// Clip the status bar to the terminal width so it never wraps onto extra
	// lines (which would push the layout past the screen on small terminals).
	status := m.renderer.NewStyle().MaxWidth(m.width).Render(m.statusBar())
	return style.Render(canvas) + "\n" + status
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
