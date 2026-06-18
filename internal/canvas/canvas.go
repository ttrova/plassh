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
	// Always return a caller-owned buffer (the model mutates it in place): pad if
	// shorter than expected (e.g. resized canvas), otherwise copy.
	grid := make([]byte, c.width*c.height)
	copy(grid, b)
	return grid, nil
}

// SetPixel writes one pixel to Redis and publishes the change.
func (c *Canvas) SetPixel(ctx context.Context, x, y, color int) error {
	offset := int64(Index(x, y, c.width))
	if err := c.rdb.SetRange(ctx, gridKey, offset, string([]byte{byte(color)})).Err(); err != nil {
		return err
	}
	return c.rdb.Publish(ctx, updateChannel, PixelUpdate{X: x, Y: y, Color: color}.Encode()).Err()
}

// SetPixels writes many pixels to Redis with one pipeline and broadcasts them as
// a single batch message — so a bulk operation (clear/fill/circle/flood/line) is
// one round-trip and one pub/sub message instead of thousands.
func (c *Canvas) SetPixels(ctx context.Context, ups []PixelUpdate) error {
	if len(ups) == 0 {
		return nil
	}
	if len(ups) == 1 {
		return c.SetPixel(ctx, ups[0].X, ups[0].Y, ups[0].Color)
	}
	pipe := c.rdb.Pipeline()
	for _, u := range ups {
		offset := int64(Index(u.X, u.Y, c.width))
		pipe.SetRange(ctx, gridKey, offset, string([]byte{byte(u.Color)}))
	}
	pipe.Publish(ctx, updateChannel, EncodeBatch(ups))
	_, err := pipe.Exec(ctx)
	return err
}

// Subscribe returns a channel of decoded pixel-update batches (a single paint is
// a batch of one). The backing goroutine stops and the channel closes when ctx
// is cancelled.
func (c *Canvas) Subscribe(ctx context.Context) <-chan []PixelUpdate {
	out := make(chan []PixelUpdate, 64)
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
				ups, err := Decode(msg.Payload)
				if err != nil {
					log.Printf("canvas: bad update payload %q: %v", msg.Payload, err)
					continue
				}
				select {
				case out <- ups:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
