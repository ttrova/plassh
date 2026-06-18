package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("CANVAS_WIDTH", "")
	t.Setenv("CANVAS_HEIGHT", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("SSH_PORT", "")
	t.Setenv("SSH_HOST_KEY", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Width != 100 || c.Height != 100 {
		t.Errorf("got %dx%d, want 100x100", c.Width, c.Height)
	}
	if c.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q", c.RedisAddr)
	}
	if c.SSHPort != "2222" {
		t.Errorf("SSHPort = %q", c.SSHPort)
	}
	if c.HostKeyPath != "./host_key" {
		t.Errorf("HostKeyPath = %q", c.HostKeyPath)
	}
}

func TestLoadDisabledCommands(t *testing.T) {
	t.Setenv("DISABLED_COMMANDS", " Clear , fill ,")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.DisabledCommands["clear"] || !c.DisabledCommands["fill"] {
		t.Errorf("DisabledCommands = %v, want clear+fill", c.DisabledCommands)
	}
	if c.DisabledCommands["tp"] {
		t.Error("tp should not be disabled")
	}
}

func TestLoadAdminAndCooldown(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("ADMIN_COMMANDS", "clear, fill")
	t.Setenv("PAINT_COOLDOWN_MS", "250")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.AdminPassword != "secret" {
		t.Errorf("AdminPassword = %q", c.AdminPassword)
	}
	if !c.AdminCommands["clear"] || !c.AdminCommands["fill"] || c.AdminAll {
		t.Errorf("AdminCommands = %v all=%v", c.AdminCommands, c.AdminAll)
	}
	if c.PaintCooldown != 250*time.Millisecond {
		t.Errorf("PaintCooldown = %v, want 250ms", c.PaintCooldown)
	}
}

func TestLoadAdminAllWildcard(t *testing.T) {
	t.Setenv("ADMIN_COMMANDS", "*")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.AdminAll {
		t.Error("expected AdminAll for '*'")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("CANVAS_WIDTH", "200")
	t.Setenv("CANVAS_HEIGHT", "50")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Width != 200 || c.Height != 50 {
		t.Errorf("got %dx%d, want 200x50", c.Width, c.Height)
	}
}

func TestLoadRejectsNonPositive(t *testing.T) {
	t.Setenv("CANVAS_WIDTH", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for width 0, got nil")
	}
}

func TestLoadRejectsNonNumeric(t *testing.T) {
	t.Setenv("CANVAS_WIDTH", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric width, got nil")
	}
}
