package render

import "testing"

func TestColorName(t *testing.T) {
	if ColorName(0) != "black" {
		t.Errorf("ColorName(0) = %q", ColorName(0))
	}
	if ColorName(1) != "red" {
		t.Errorf("ColorName(1) = %q", ColorName(1))
	}
	if ColorName(7) != "white" {
		t.Errorf("ColorName(7) = %q", ColorName(7))
	}
}

func TestColorCount(t *testing.T) {
	if NumColors != 8 {
		t.Errorf("NumColors = %d, want 8", NumColors)
	}
}

func TestNextColorCycles(t *testing.T) {
	if NextColor(7) != 0 {
		t.Errorf("NextColor(7) = %d, want 0", NextColor(7))
	}
	if NextColor(3) != 4 {
		t.Errorf("NextColor(3) = %d, want 4", NextColor(3))
	}
}
