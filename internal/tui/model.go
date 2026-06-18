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
	drawing          bool // continuous draw mode: movement paints a trail

	width, height int // terminal size in cells
	remotes       map[string]remote

	renderer  *lipgloss.Renderer
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
		cmd := m.paint()
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
	case 'd':
		// Toggle continuous draw mode. Turning it on drops a pixel at the cursor
		// so a stroke starts where you are.
		m.drawing = !m.drawing
		if m.drawing {
			cmd := m.paint()
			return *m, cmd
		}
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
// draw mode is active — paints the pixel the cursor just moved onto, so moving
// lays down a continuous trail.
func (m *Model) afterMove() tea.Cmd {
	m.recenter()
	cmds := []tea.Cmd{m.announce()}
	if m.drawing {
		cmds = append(cmds, m.paint())
	}
	return tea.Batch(cmds...)
}

// paint applies the current color at the cursor optimistically and writes through.
func (m *Model) paint() tea.Cmd {
	if !m.inBounds(m.cursorX, m.cursorY) {
		return nil
	}
	m.grid[m.cursorY*m.canvasW+m.cursorX] = byte(m.selectedColor)
	if m.painter == nil {
		return nil
	}
	x, y, c := m.cursorX, m.cursorY, m.selectedColor
	return func() tea.Msg {
		_ = m.painter.SetPixel(m.ctx, x, y, c)
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

func (m *Model) recenter() {
	pw := render.VisiblePixelWidth(m.width)
	ph := render.VisiblePixelHeight(m.height)
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
	pw := render.VisiblePixelWidth(m.width)
	ph := render.VisiblePixelHeight(m.height)

	canvas := render.Canvas(render.View{
		Renderer: m.renderer,
		Grid:     m.grid, Width: m.canvasW, Height: m.canvasH,
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

	return style.Render(canvas) + "\n" + m.statusBar()
}

func (m Model) remoteCursors() []render.RemoteCursor {
	out := make([]render.RemoteCursor, 0, len(m.remotes))
	for _, r := range m.remotes {
		out = append(out, render.RemoteCursor{X: r.x, Y: r.y, Color: r.color})
	}
	return out
}

func (m Model) statusBar() string {
	swatch := m.renderer.NewStyle().Foreground(render.ColorAt(m.selectedColor)).Render("█")
	draw := ""
	if m.drawing {
		draw = " │ " + m.renderer.NewStyle().Foreground(render.ColorAt(2)).Bold(true).Render("DRAW")
	}
	return fmt.Sprintf(
		"You: %s │ Color: %s %s (%d/8) │ Pos: %d,%d │ Users: %d%s │ %s",
		m.name, swatch, render.ColorName(m.selectedColor), m.selectedColor+1,
		m.cursorX, m.cursorY, len(m.remotes)+1, draw,
		"1-8/Tab color · Space dab · d draw · hjkl move · q quit",
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
