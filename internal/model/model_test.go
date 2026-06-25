package model

import (
	"reflect"
	"testing"
)

func TestNormalizePorts(t *testing.T) {
	got := NormalizePorts([]int{8080, 3000, 8080, 0, 70000, 443, -5})
	want := []int{443, 3000, 8080}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizePorts = %v; want %v", got, want)
	}
}

func TestPortsString(t *testing.T) {
	if got := (Host{OpenPorts: []int{80, 443}}).PortsString(); got != "80, 443" {
		t.Errorf("PortsString = %q; want '80, 443'", got)
	}
	if got := (Host{}).PortsString(); got != "" {
		t.Errorf("empty PortsString = %q; want ''", got)
	}
}

func TestDefaultProfile(t *testing.T) {
	p := DefaultProfile()
	if p.IntervalSec != 60 || p.Concurrency != 100 || len(p.Ports) != 2 {
		t.Errorf("unexpected defaults: %+v", p)
	}
}
