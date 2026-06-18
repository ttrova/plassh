// Package ssh wires the wish SSH server to a per-session Bubble Tea program.
package ssh

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"plassh/internal/canvas"
	"plassh/internal/config"
	"plassh/internal/presence"
	"plassh/internal/tui"
)

// NewServer builds the wish SSH server bound to the given canvas/presence layers.
func NewServer(cfg config.Config, cv *canvas.Canvas, pr *presence.Presence) (*ssh.Server, error) {
	return wish.NewServer(
		wish.WithAddress(":"+cfg.SSHPort),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithMiddleware(
			bm.Middleware(handler(cfg, cv, pr)),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
}

func handler(cfg config.Config, cv *canvas.Canvas, pr *presence.Presence) bm.Handler {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		ctx := s.Context()
		name := presence.SanitizeName(s.User())
		id := randomID()

		grid, err := cv.Load(ctx)
		if err != nil {
			log.Printf("ssh: load grid: %v", err)
			wish.Println(s, "canvas unavailable, try again later")
			return nil, nil
		}

		presCh := bridgePresence(ctx, pr.Subscribe(ctx))
		pixCh := bridgePixels(ctx, cv.Subscribe(ctx))
		existing, _ := pr.LoadAll(ctx)

		m := tui.New(tui.Deps{
			Ctx:       ctx,
			Renderer:  bm.MakeRenderer(s), // per-session color profile from the client's TERM
			Width:     cfg.Width,
			Height:    cfg.Height,
			Grid:      grid,
			Name:      name,
			ID:        id,
			Painter:   cv,
			Announcer: pr,
			Pixels:    pixCh,
			Presence:  presCh,
		})
		m = tui.WithExisting(m, existing, id)

		go func() {
			<-ctx.Done()
			bg := context.Background()
			_ = pr.Remove(bg, id)
			_ = pr.Publish(bg, presence.Update{ID: id, Gone: true})
		}()

		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}
}

func bridgePixels(ctx context.Context, in <-chan canvas.PixelUpdate) <-chan tui.PixelUpdateMsg {
	out := make(chan tui.PixelUpdateMsg, 64)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case u, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- tui.PixelUpdateMsg{X: u.X, Y: u.Y, Color: u.Color}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func bridgePresence(ctx context.Context, in <-chan presence.Update) <-chan tui.PresenceEvent {
	out := make(chan tui.PresenceEvent, 64)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case u, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- tui.NewPresenceEvent(u):
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
