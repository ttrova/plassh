package main

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"

	"plassh/internal/canvas"
	"plassh/internal/config"
	"plassh/internal/presence"
	sshserver "plassh/internal/ssh"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}

	cv := canvas.New(rdb, cfg.Width, cfg.Height)
	if err := cv.Init(ctx); err != nil {
		log.Fatalf("canvas init: %v", err)
	}
	pr := presence.New(rdb)

	srv, err := sshserver.NewServer(cfg, cv, pr)
	if err != nil {
		log.Fatalf("ssh server: %v", err)
	}

	log.Printf("listening on :%s (canvas %dx%d, redis %s)", cfg.SSHPort, cfg.Width, cfg.Height, cfg.RedisAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
