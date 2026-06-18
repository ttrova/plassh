package presence

import "testing"

func TestEncodeDecodeUpdate(t *testing.T) {
	in := Update{ID: "abc123", X: 12, Y: 40, Color: 5, Name: "alice"}
	out, err := Decode(in.Encode())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != in {
		t.Errorf("round trip: got %+v, want %+v", out, in)
	}
}

func TestEncodeDecodeGone(t *testing.T) {
	in := Update{ID: "abc123", Gone: true}
	out, err := Decode(in.Encode())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Gone || out.ID != "abc123" {
		t.Errorf("got %+v", out)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"alice":                       "alice",
		"":                            "anon",
		"   ":                         "anon",
		"a|b\tc":                      "abc",
		"verylongusernamethatexceeds": "verylongusername", // capped at 16
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
