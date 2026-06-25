package updates

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.1.1", "1.1.0", true},
		{"1.2.0", "1.1.9", true},
		{"2.0.0", "1.9.9", true},
		{"1.1.0", "1.1.0", false},
		{"1.0.0", "1.1.0", false},
		{"1.1.0", "1.1.1", false},
		{"1.1.1", "v1.1.0", true}, // tolerate stray leading v already stripped upstream
	}
	for _, c := range cases {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q, %q) = %v; want %v", c.a, c.b, got, c.want)
		}
	}
}
