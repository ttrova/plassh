package canvas

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

const (
	gridKey       = "canvas:grid"
	updateChannel = "canvas:updates"
)

// Canvas is the Redis-backed pixel grid plus its broadcast channel.
type Canvas struct {
	rdb    *redis.Client
	width  int
	height int
}

// New returns a Canvas bound to the given Redis client and dimensions.
func New(rdb *redis.Client, width, height int) *Canvas {
	return &Canvas{rdb: rdb, width: width, height: height}
}

// Init ensures the grid key exists, zero-filled, without clobbering existing state.
func (c *Canvas) Init(ctx context.Context) error {
	zero := make([]byte, c.width*c.height)
	return c.rdb.SetNX(ctx, gridKey, string(zero), 0).Err()
}

// Load returns a copy of the full grid as one byte per pixel.
func (c *Canvas) Load(ctx context.Context) ([]byte, error) {
	b, err := c.rdb.Get(ctx, gridKey).Bytes()
	if err == redis.Nil {
		return make([]byte, c.width*c.height), nil
	}
	if err != nil {
		return nil, err
	}
	// Defensive: pad if shorter than expected (e.g. resized canvas).
	if len(b) < c.width*c.height {
		padded := make([]byte, c.width*c.height)
		copy(padded, b)
		return padded, nil
	}
	return b, nil
}

// SetPixel writes one pixel to Redis and publishes the change.
func (c *Canvas) SetPixel(ctx context.Context, x, y, color int) error {
	offset := int64(Index(x, y, c.width))
	if err := c.rdb.SetRange(ctx, gridKey, offset, string([]byte{byte(color)})).Err(); err != nil {
		return err
	}
	return c.rdb.Publish(ctx, updateChannel, PixelUpdate{X: x, Y: y, Color: color}.Encode()).Err()
}

// Subscribe returns a channel of decoded pixel updates. The backing goroutine
// stops and the channel closes when ctx is cancelled.
func (c *Canvas) Subscribe(ctx context.Context) <-chan PixelUpdate {
	out := make(chan PixelUpdate, 64)
	pubsub := c.rdb.Subscribe(ctx, updateChannel)
	go func() {
		defer close(out)
		defer pubsub.Close()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				u, err := Decode(msg.Payload)
				if err != nil {
					log.Printf("canvas: bad update payload %q: %v", msg.Payload, err)
					continue
				}
				select {
				case out <- u:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
