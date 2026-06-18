package presence

import (
	"testing"
	"unicode/utf8"
)

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
		"éééééééééééééééééééé": "éééééééééééééééé", // 20 multibyte runes -> 16
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeNameStaysValidUTF8(t *testing.T) {
	// A name whose 16th rune boundary does not fall on byte 16 must not be
	// sliced mid-rune (would corrupt the status bar / presence payload).
	got := SanitizeName("théàuuuuuuuuuuuuuuuuuuu")
	if !utf8.ValidString(got) {
		t.Errorf("SanitizeName produced invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n > maxNameLen {
		t.Errorf("rune count %d exceeds cap %d", n, maxNameLen)
	}
}
