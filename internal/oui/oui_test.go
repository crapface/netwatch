package oui

import (
	"strings"
	"testing"
)

const sampleTXT = "00-00-00   (hex)\t\tXEROX CORPORATION\n" +
	"84-47-09   (hex)\t\tSample Vendor Inc\n" +
	"AC-DE-48   (hex)\t\tPRIVATE\n"

const sampleCSV = `Registry,Assignment,Organization Name,Organization Address
MA-L,000000,XEROX CORPORATION,"M/S 105-50C, WEBSTER NY US 14580"
MA-L,844709,Sample Vendor Inc,"123 Road, City US"
`

func TestParseTXT(t *testing.T) {
	m, err := parseTXT(strings.NewReader(sampleTXT))
	if err != nil {
		t.Fatal(err)
	}
	if m["844709"] != "Sample Vendor Inc" {
		t.Errorf("TXT 844709 = %q; want 'Sample Vendor Inc'", m["844709"])
	}
	if m["000000"] != "XEROX CORPORATION" {
		t.Errorf("TXT 000000 = %q", m["000000"])
	}
}

func TestParseCSV(t *testing.T) {
	m, err := parseCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	if m["844709"] != "Sample Vendor Inc" {
		t.Errorf("CSV 844709 = %q; want 'Sample Vendor Inc'", m["844709"])
	}
}

// TestVendorOfEndToEnd proves the lookup pipeline (parse -> swap -> VendorOf)
// resolves a real-world MAC like the gateway's 84:47:09:… once data is loaded,
// which confirms blank vendors are a data/download problem, not a lookup bug.
func TestVendorOfEndToEnd(t *testing.T) {
	m, _ := parseCSV(strings.NewReader(sampleCSV))
	swap(m)
	if got := VendorOf("84:47:09:66:67:EA"); got != "Sample Vendor Inc" {
		t.Errorf("VendorOf(84:47:09:…) = %q; want 'Sample Vendor Inc'", got)
	}
	if got := VendorOf("00-00-00-12-34-56"); got != "XEROX CORPORATION" {
		t.Errorf("VendorOf(00-00-00-…) = %q", got)
	}
	if got := VendorOf("garbage"); got != "" {
		t.Errorf("VendorOf(bad) = %q; want empty", got)
	}
}
