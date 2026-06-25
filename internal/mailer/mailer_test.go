package mailer

import "testing"

func TestRecipients(t *testing.T) {
	got := Recipients("a@x.com, b@y.com;c@z.com  d@w.com")
	if len(got) != 4 {
		t.Fatalf("want 4 recipients, got %d: %v", len(got), got)
	}
	if got[0] != "a@x.com" || got[3] != "d@w.com" {
		t.Errorf("parse mismatch: %v", got)
	}
	if len(Recipients("   ")) != 0 {
		t.Errorf("blank should yield no recipients")
	}
}
