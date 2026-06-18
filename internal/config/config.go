// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime settings for the server.
type Config struct {
	Width       int
	Height      int
	RedisAddr   string
	SSHPort     string
	HostKeyPath string
}

// Load reads configuration from the environment, applying defaults and
// validating that the canvas dimensions are positive.
func Load() (Config, error) {
	c := Config{
		Width:       100,
		Height:      100,
		RedisAddr:   getDefault("REDIS_ADDR", "localhost:6379"),
		SSHPort:     getDefault("SSH_PORT", "2222"),
		HostKeyPath: getDefault("SSH_HOST_KEY", "./host_key"),
	}

	var err error
	if c.Width, err = getInt("CANVAS_WIDTH", 100); err != nil {
		return Config{}, err
	}
	if c.Height, err = getInt("CANVAS_HEIGHT", 100); err != nil {
		return Config{}, err
	}
	if c.Width <= 0 || c.Height <= 0 {
		return Config{}, fmt.Errorf("canvas dimensions must be positive, got %dx%d", c.Width, c.Height)
	}
	return c, nil
}

func getDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}
