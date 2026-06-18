package canvas

import "testing"

func TestIndex(t *testing.T) {
	if got := Index(3, 2, 10); got != 23 {
		t.Errorf("Index(3,2,10) = %d, want 23", got)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := PixelUpdate{X: 12, Y: 40, Color: 5}
	out, err := Decode(in.Encode())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != in {
		t.Errorf("round trip: got %+v, want %+v", out, in)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode("not,a,number"); err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, err := Decode("1,2"); err == nil {
		t.Fatal("expected error for too few fields, got nil")
	}
}
