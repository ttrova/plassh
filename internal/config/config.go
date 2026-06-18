// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings for the server.
type Config struct {
	Width       int
	Height      int
	RedisAddr   string
	SSHPort     string
	HostKeyPath string
	// DisabledCommands is the set of slash-command names hard-disabled for everyone
	// (via DISABLED_COMMANDS, comma-separated, e.g. "clear,fill").
	DisabledCommands map[string]bool
	// AdminPassword enables /login; empty disables admin entirely.
	AdminPassword string
	// AdminCommands are command names that require admin (via ADMIN_COMMANDS). If
	// AdminAll is set ("*" in the list), every command requires admin.
	AdminCommands map[string]bool
	AdminAll      bool
	// PaintCooldown is the minimum delay between paint actions (via
	// PAINT_COOLDOWN_MS); 0 disables rate limiting. Admins are exempt.
	PaintCooldown time.Duration
}

// Load reads configuration from the environment, applying defaults and
// validating that the canvas dimensions are positive.
func Load() (Config, error) {
	c := Config{
		Width:            100,
		Height:           100,
		RedisAddr:        getDefault("REDIS_ADDR", "localhost:6379"),
		SSHPort:          getDefault("SSH_PORT", "2222"),
		HostKeyPath:      getDefault("SSH_HOST_KEY", "./host_key"),
		DisabledCommands: parseDisabled(os.Getenv("DISABLED_COMMANDS")),
		AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
	}
	c.AdminCommands, c.AdminAll = parseAdminCommands(os.Getenv("ADMIN_COMMANDS"))

	var err error
	if c.Width, err = getInt("CANVAS_WIDTH", 100); err != nil {
		return Config{}, err
	}
	if c.Height, err = getInt("CANVAS_HEIGHT", 100); err != nil {
		return Config{}, err
	}
	cooldownMs, err := getInt("PAINT_COOLDOWN_MS", 0)
	if err != nil {
		return Config{}, err
	}
	if cooldownMs < 0 {
		return Config{}, fmt.Errorf("PAINT_COOLDOWN_MS must be >= 0, got %d", cooldownMs)
	}
	c.PaintCooldown = time.Duration(cooldownMs) * time.Millisecond

	if c.Width <= 0 || c.Height <= 0 {
		return Config{}, fmt.Errorf("canvas dimensions must be positive, got %dx%d", c.Width, c.Height)
	}
	return c, nil
}

// parseAdminCommands turns a comma-separated list into a lowercased name set,
// returning all=true if it contains "*" (every command requires admin).
func parseAdminCommands(v string) (map[string]bool, bool) {
	set := make(map[string]bool)
	all := false
	for _, part := range strings.Split(v, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		switch {
		case name == "":
		case name == "*":
			all = true
		default:
			set[name] = true
		}
	}
	return set, all
}

// parseDisabled turns a comma-separated list into a lowercased name set.
func parseDisabled(v string) map[string]bool {
	out := make(map[string]bool)
	for _, part := range strings.Split(v, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			out[name] = true
		}
	}
	return out
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
