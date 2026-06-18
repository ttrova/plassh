package canvas

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

func TestInitAndLoadZeroGrid(t *testing.T) {
	ctx := context.Background()
	rdb := newTestClient(t)
	c := New(rdb, 4, 3)

	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	grid, err := c.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(grid) != 12 {
		t.Fatalf("len(grid) = %d, want 12", len(grid))
	}
	for i, b := range grid {
		if b != 0 {
			t.Fatalf("grid[%d] = %d, want 0", i, b)
		}
	}
}

func TestSetPixelPersists(t *testing.T) {
	ctx := context.Background()
	rdb := newTestClient(t)
	c := New(rdb, 4, 3)
	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := c.SetPixel(ctx, 1, 2, 5); err != nil {
		t.Fatalf("SetPixel: %v", err)
	}
	grid, err := c.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if grid[Index(1, 2, 4)] != 5 {
		t.Errorf("grid at (1,2) = %d, want 5", grid[Index(1, 2, 4)])
	}
}

func TestSetPixelPublishes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rdb := newTestClient(t)
	c := New(rdb, 4, 3)
	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	updates := c.Subscribe(ctx)
	time.Sleep(50 * time.Millisecond) // let the subscription register

	if err := c.SetPixel(ctx, 3, 1, 7); err != nil {
		t.Fatalf("SetPixel: %v", err)
	}

	select {
	case batch := <-updates:
		if len(batch) != 1 || batch[0] != (PixelUpdate{X: 3, Y: 1, Color: 7}) {
			t.Errorf("got %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update")
	}
}

func TestSetPixelsBatchPublishes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rdb := newTestClient(t)
	c := New(rdb, 4, 3)
	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	updates := c.Subscribe(ctx)
	time.Sleep(50 * time.Millisecond)

	ups := []PixelUpdate{{X: 0, Y: 0, Color: 1}, {X: 1, Y: 0, Color: 2}, {X: 2, Y: 0, Color: 3}}
	if err := c.SetPixels(ctx, ups); err != nil {
		t.Fatalf("SetPixels: %v", err)
	}

	select {
	case batch := <-updates:
		if len(batch) != 3 {
			t.Fatalf("got %d updates in one message, want 3", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for batch")
	}

	grid, err := c.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if grid[0] != 1 || grid[1] != 2 || grid[2] != 3 {
		t.Errorf("batch not persisted: %v", grid[:3])
	}
}
