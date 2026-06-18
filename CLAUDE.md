# CLAUDE.md

## Project: SSH Collaborative Pixel Canvas (r/place-like)

A collaborative pixel-art canvas accessible over SSH. Multiple users connect
simultaneously, move a cursor, and paint pixels in real time. Think "r/place
in your terminal", with no cooldown (collaborative paint).

This file is the source of truth for the MVP scope. Stay within it.

---

## Tech Stack

- **Language:** Go (latest stable)
- **SSH server:** `github.com/charmbracelet/wish`
- **TUI framework:** `github.com/charmbracelet/bubbletea` + `lipgloss`
- **State / pub-sub:** Redis (state storage + broadcast queue)
- **Deployment:** Docker (multi-stage build)

---

## Core Concept / Rendering

- **Half-block rendering:** each terminal cell = 2 vertical pixels.
  - Use `▀` (U+2580, upper half block).
  - **Foreground color** = top pixel.
  - **Background color** = bottom pixel.
- This doubles vertical resolution: a canvas of `H` pixels uses `H/2` rows.

---

## Canvas

- **Configurable size** via environment variables:
  - `CANVAS_WIDTH` (pixels, default: `100`)
  - `CANVAS_HEIGHT` (pixels, default: `100`)
- The canvas is almost always larger than the terminal viewport.
- **Scrolling:** horizontal + vertical. The viewport follows the cursor;
  when the cursor reaches a viewport edge, the view scrolls.
- **Borders:** draw clear canvas borders so users know where the edges are.
  The cursor cannot move outside the canvas bounds.

---

## Colors

- **8 colors** for the MVP (standard ANSI: black, red, green, yellow, blue,
  magenta, cyan, white).
- Selection:
  - Number keys `1`–`8` select a color directly.
  - `Tab` cycles to the next color.
- Show the currently selected color in a status bar.

---

## Controls (keyboard only — NO mouse in MVP)

| Key                     | Action                          |
|-------------------------|---------------------------------|
| Arrow keys / `h` `j` `k` `l` | Move cursor (left/down/up/right) |
| `Space`                 | Paint current color at cursor   |
| `1`–`8`                 | Select color                    |
| `Tab`                   | Cycle to next color             |
| `q` / `Ctrl+C`          | Quit / disconnect               |

`hjkl` mapping: `h`=left, `j`=down, `k`=up, `l`=right (vim-style).

---

## Multi-user / Real-time Sync

- All connected sessions see each other's pixels in **real time**.
- **Redis is used both as state store and broadcast bus:**
  - **State:** the canvas grid is stored in Redis (survives restarts → persistence).
  - **Broadcast:** when a user paints a pixel, publish the change so all
    sessions update. Use Redis Pub/Sub (or a stream) as the queue.
- Each session subscribes on connect, applies incoming pixel updates to its
  local view, and re-renders.
- Handle concurrent paints safely (last write wins is acceptable for MVP).

### Suggested Redis model
- Canvas pixels: a single key (e.g. a byte array / hash) keyed by pixel index
  `y*width + x` storing a color value `0–7`. Pick whatever is simplest and
  efficient enough for the MVP.
- Pub/Sub channel: e.g. `canvas:updates`, message = `{x, y, color}`.

---

## Authentication

- **Anonymous SSH access.** Accept any connection, no password, no public key
  required. (Public collaborative canvas.)
- Generate or mount a host key for the SSH server.

---

## Persistence

- State lives in Redis.
- Redis persistence (RDB/AOF) is what we back up. The Go app itself is
  stateless beyond live sessions.

---

## Docker

- Multi-stage build: `golang:alpine` (build) → minimal final image with the binary.
- App connects to Redis via env var (e.g. `REDIS_ADDR`, default `localhost:6379`).
- Expose the SSH port (e.g. `2222`).
- Provide a `docker-compose.yml` with two services: the Go SSH app + Redis
  (with a volume for Redis persistence).

### Environment variables (summary)
| Var             | Default          | Description              |
|-----------------|------------------|--------------------------|
| `CANVAS_WIDTH`  | `100`            | Canvas width in pixels   |
| `CANVAS_HEIGHT` | `100`            | Canvas height in pixels  |
| `REDIS_ADDR`    | `localhost:6379` | Redis address            |
| `SSH_PORT`      | `2222`           | SSH listen port          |
| `SSH_HOST_KEY`  | (path)           | Host key file location   |

---

## MVP Scope — IN

- Half-block rendering, 8 ANSI colors.
- Configurable canvas size, scrolling viewport, visible borders.
- Keyboard cursor (arrows + hjkl), paint with space.
- Color selection (1–8, Tab).
- Real-time multi-user sync via Redis.
- Anonymous SSH.
- Redis-backed persistence + Docker/Compose setup.

## MVP Scope — OUT (do not build yet)

- ❌ Mouse support.
- ❌ Cooldown / rate limiting / timers.
- ❌ Zoom levels (single zoom only).
- ❌ Truecolor / 256-color palettes.
- ❌ User accounts / names / auth.
- ❌ Undo, history, replay.
- ❌ Chat.

---

## Coding Guidelines

- Keep it simple and readable; this is an MVP.
- Idiomatic Go; handle errors explicitly.
- Isolate concerns: SSH/session layer, Bubble Tea model, Redis layer, render layer.
- Don't redraw the whole screen on every keystroke if avoidable, but don't
  over-optimize either — correctness first.
- Gracefully handle client disconnects (unsubscribe, clean up goroutines).

