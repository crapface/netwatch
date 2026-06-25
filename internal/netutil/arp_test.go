package netutil

import "testing"

func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"00-1a-2b-3c-4d-5e": "00:1A:2B:3C:4D:5E",
		"00:1A:2B:3C:4D:5E": "00:1A:2B:3C:4D:5E",
		"001a2b3c4d5e":      "00:1A:2B:3C:4D:5E",
		"garbage":           "",
	}
	for in, want := range cases {
		if got := NormalizeMAC(in); got != want {
			t.Errorf("NormalizeMAC(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestOUIKey(t *testing.T) {
	if got := OUIKey("00:1a:2b:3c:4d:5e"); got != "001A2B" {
		t.Errorf("OUIKey = %q; want 001A2B", got)
	}
	if got := OUIKey("zz"); got != "" {
		t.Errorf("OUIKey(bad) = %q; want empty", got)
	}
}
