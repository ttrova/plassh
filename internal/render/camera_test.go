package render

import "testing"

func TestCameraKeepsCursorVisible(t *testing.T) {
	// viewport 10 wide, canvas 100, cursor far right of current camera.
	cam := CameraFor(50, 0, 10, 100)
	if 50 < cam || 50 >= cam+10 {
		t.Errorf("cursor 50 not visible: cam=%d viewport=10", cam)
	}
}

func TestCameraClampsToZero(t *testing.T) {
	if got := CameraFor(0, 5, 10, 100); got != 0 {
		t.Errorf("CameraFor at left edge = %d, want 0", got)
	}
}

func TestCameraClampsToMax(t *testing.T) {
	// canvas 100, viewport 10 -> max camera is 90.
	if got := CameraFor(99, 0, 10, 100); got != 90 {
		t.Errorf("CameraFor at right edge = %d, want 90", got)
	}
}

func TestCameraViewportLargerThanCanvas(t *testing.T) {
	// viewport bigger than canvas -> camera pinned at 0.
	if got := CameraFor(5, 0, 200, 100); got != 0 {
		t.Errorf("CameraFor oversized viewport = %d, want 0", got)
	}
}

func TestVisiblePixels(t *testing.T) {
	// 24 rows total, chrome = 4 (header+2 border+status) -> 20 usable -> 40 pixel rows.
	if got := VisiblePixelHeight(24); got != 40 {
		t.Errorf("VisiblePixelHeight(24) = %d, want 40", got)
	}
	// 80 cols total, chrome = 2 -> 78 usable pixel columns.
	if got := VisiblePixelWidth(80); got != 78 {
		t.Errorf("VisiblePixelWidth(80) = %d, want 78", got)
	}
}
