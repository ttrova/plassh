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

| Key                          | Action                |
|------------------------------|-----------------------|
| Arrow keys / `h` `j` `k` `l` | Move cursor           |
| `Space`                      | Paint selected color  |
| `1`–`8`                      | Select color          |
| `Tab`                        | Cycle color           |
| `q` / `Ctrl+C`               | Quit                  |

Your cursor is a `+` in your selected color. Other users appear as a colored
half-shade square (`🮎`/`🮏`) at their pixel.

## Configuration

| Var             | Default          | Description            |
|-----------------|------------------|------------------------|
| `CANVAS_WIDTH`  | `100`            | Canvas width (pixels)  |
| `CANVAS_HEIGHT` | `100`            | Canvas height (pixels) |
| `REDIS_ADDR`    | `localhost:6379` | Redis address          |
| `SSH_PORT`      | `2222`           | SSH listen port        |
| `SSH_HOST_KEY`  | `./host_key`     | Host key path (auto-generated if missing) |

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
