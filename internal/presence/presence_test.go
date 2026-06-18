package presence

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestPublishAndSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rdb := newTestClient(t)
	p := New(rdb)

	updates := p.Subscribe(ctx)
	time.Sleep(50 * time.Millisecond)

	u := Update{ID: "u1", X: 5, Y: 6, Color: 2, Name: "bob"}
	if err := p.Publish(ctx, u); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case got := <-updates:
		if got != u {
			t.Errorf("got %+v, want %+v", got, u)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestLoadAllReturnsExisting(t *testing.T) {
	ctx := context.Background()
	rdb := newTestClient(t)
	p := New(rdb)

	if err := p.Touch(ctx, Update{ID: "u1", X: 1, Y: 2, Color: 3, Name: "a"}); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := p.Touch(ctx, Update{ID: "u2", X: 4, Y: 5, Color: 6, Name: "b"}); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	all, err := p.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(all) = %d, want 2", len(all))
	}
}

func TestRemoveDeletesKey(t *testing.T) {
	ctx := context.Background()
	rdb := newTestClient(t)
	p := New(rdb)
	if err := p.Touch(ctx, Update{ID: "u1", X: 1, Y: 2, Color: 3, Name: "a"}); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := p.Remove(ctx, "u1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	all, err := p.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("len(all) = %d, want 0", len(all))
	}
}
