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
	if len(out) != 1 || out[0] != in {
		t.Errorf("round trip: got %+v, want [%+v]", out, in)
	}
}

func TestEncodeDecodeBatch(t *testing.T) {
	in := []PixelUpdate{{X: 1, Y: 2, Color: 3}, {X: 4, Y: 5, Color: 6}, {X: 7, Y: 8, Color: 0}}
	out, err := Decode(EncodeBatch(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("got %d updates, want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("update %d: got %+v, want %+v", i, out[i], in[i])
		}
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
