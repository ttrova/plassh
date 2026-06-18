# plassh — SSH Collaborative Pixel Canvas

An r/place-style collaborative pixel canvas in your terminal, over SSH. Multiple
people connect anonymously, move a cursor, and paint pixels that everyone sees in
real time — no cooldown.

## Run

```bash
docker compose up -d
ssh -p 2222 yourname@localhost
```

The name you connect with (`yourname@`) is shown in your status bar and to other
users. No password or key is required.

## Controls

| Key                          | Action                                  |
|------------------------------|-----------------------------------------|
| Arrow keys / `h` `j` `k` `l` | Move cursor                             |
| `Space`                      | Paint one pixel at the cursor (a dab)   |
| `d`                          | Toggle draw mode (paint while you move) |
| `1`–`8`                      | Select color                            |
| `Tab`                        | Cycle color                             |
| `/`                          | Open the command line (see below)       |
| `q` / `Ctrl+C`               | Quit                                    |

### Commands

Press `/` to open a command line in the status bar, type, then **Enter** to run
(**Esc** cancels, **Backspace** edits):

| Command               | Action                                                       |
|-----------------------|--------------------------------------------------------------|
| `/tp x y`             | Teleport the cursor to pixel `x,y` (clamped to the canvas)   |
| `/circle <r>`         | Filled disk of radius `r` (max 10) in the current color      |
| `/fill x1 y1 x2 y2`   | Fill the rectangle between two corners in the current color  |
| `/line x1 y1 x2 y2`   | Draw a line between two points in the current color          |
| `/flood`              | Flood-fill the region under the cursor with the current color |
| `/clear`              | Clear the whole canvas                                       |
| `/undo [n]`           | Undo your last `n` actions (default 1, max 10)               |
| `/redo [n]`           | Redo previously undone actions (default 1, max 10)           |
| `/help`               | Show the command list                                        |

An action is one dab, one `/circle`/`/fill`/`/line`/`/clear`, or one draw stroke.
Undo is per-session and in-memory: it restores each affected pixel to the color
it had before your action (last-write-wins, so it can overwrite others' later edits).

Commands can be disabled by the operator via the `DISABLED_COMMANDS` environment
variable (comma-separated names, e.g. `DISABLED_COMMANDS=clear,fill`).

Your cursor — and every other user's — is a top/bottom half-shade square
(`🮎`/`🮏`) in that user's selected color, sitting on its pixel. With **draw mode**
on (`d`), every cursor move lays down a pixel, so you can drag out lines and
shapes; the status bar shows `DRAW` while it's active. Press `d` again to stop.

## Configuration

| Var             | Default          | Description            |
|-----------------|------------------|------------------------|
| `CANVAS_WIDTH`  | `100`            | Canvas width (pixels)  |
| `CANVAS_HEIGHT` | `100`            | Canvas height (pixels) |
| `REDIS_ADDR`    | `localhost:6379` | Redis address          |
| `SSH_PORT`      | `2222`           | SSH listen port        |
| `SSH_HOST_KEY`  | `./host_key`     | Host key path (auto-generated if missing) |
| `DISABLED_COMMANDS` | (none)       | Comma-separated slash commands to disable |

State lives in Redis (canvas grid + presence), so it survives app restarts as long
as the Redis volume persists.

## Architecture

- `internal/config` — environment configuration.
- `internal/canvas` — Redis-backed pixel grid (`canvas:grid` byte array) + paint
  broadcast (`canvas:updates` pub/sub).
- `internal/presence` — live cursor registry (`presence:<id>` keys with TTL) +
  cursor broadcast (`canvas:presence` pub/sub).
- `internal/render` — pure rendering: color palette, per-cell decision, camera math,
  viewport composition.
- `internal/tui` — per-session Bubble Tea model (movement, paint, presence, view).
- `internal/ssh` — wish SSH server wiring each session to a Bubble Tea program.

## Development

```bash
go test ./...        # run the test suite
go build ./...       # build all packages
go run ./cmd/server  # run locally (needs Redis at REDIS_ADDR)
```

## Notes & limitations

- Remote-cursor glyphs (`🮎`/`🮏`) require a client terminal font with **Symbols for
  Legacy Computing** (Unicode 13+). On older fonts they render as a tofu box.
- Usernames are **unverified display labels**, not authenticated identity — anyone can
  connect as any name.
- Concurrent paints are last-write-wins.
