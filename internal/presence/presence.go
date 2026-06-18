package presence

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "presence:"
	channel   = "canvas:presence"
	// TTL must comfortably exceed the heartbeat interval (see tui).
	TTL = 10 * time.Second
)

// Presence is the Redis-backed live cursor registry plus its broadcast channel.
type Presence struct {
	rdb *redis.Client
}

// New returns a Presence bound to the given Redis client.
func New(rdb *redis.Client) *Presence {
	return &Presence{rdb: rdb}
}

// Touch writes/refreshes the user's presence key with a fresh TTL.
func (p *Presence) Touch(ctx context.Context, u Update) error {
	return p.rdb.Set(ctx, keyPrefix+u.ID, u.Encode(), TTL).Err()
}

// Publish broadcasts a presence update (move/join or gone) to all subscribers.
func (p *Presence) Publish(ctx context.Context, u Update) error {
	return p.rdb.Publish(ctx, channel, u.Encode()).Err()
}

// Remove deletes the user's presence key (used on clean disconnect).
func (p *Presence) Remove(ctx context.Context, id string) error {
	return p.rdb.Del(ctx, keyPrefix+id).Err()
}

// LoadAll returns the presence of every currently-known user. Uses KEYS, which
// is acceptable at MVP scale (tens of concurrent users).
func (p *Presence) LoadAll(ctx context.Context) ([]Update, error) {
	keys, err := p.rdb.Keys(ctx, keyPrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	var out []Update
	for _, k := range keys {
		val, err := p.rdb.Get(ctx, k).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		u, err := Decode(val)
		if err != nil {
			log.Printf("presence: bad value for %q: %v", k, err)
			continue
		}
		out = append(out, u)
	}
	return out, nil
}

// Subscribe returns a channel of decoded presence updates. The backing goroutine
// stops and the channel closes when ctx is cancelled.
func (p *Presence) Subscribe(ctx context.Context) <-chan Update {
	out := make(chan Update, 64)
	pubsub := p.rdb.Subscribe(ctx, channel)
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
					log.Printf("presence: bad payload %q: %v", msg.Payload, err)
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
