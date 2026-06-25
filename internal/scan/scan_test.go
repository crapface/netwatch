package scan

import (
	"context"
	"net"
	"strconv"
	"testing"
)

func TestHostCount(t *testing.T) {
	cases := map[string]int{
		"192.168.1.0/24": 254,
		"10.0.0.0/30":    2,
		"10.0.0.0/31":    2,
		"10.0.0.1/32":    1,
		"172.16.0.0/16":  65534,
	}
	for cidr, want := range cases {
		got, err := HostCount(cidr)
		if err != nil || got != want {
			t.Errorf("HostCount(%q) = %d, %v; want %d", cidr, got, err, want)
		}
	}
	if _, err := HostCount("10.0.0.0/6"); err == nil {
		t.Errorf("expected error for oversized range /6")
	}
}

func TestRunFindsOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, ps, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(ps)

	hosts, err := Run(context.Background(), "127.0.0.1/32", Config{
		Ports: []int{port}, Concurrency: 4, TimeoutMs: 400, Retries: 0,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].IP != "127.0.0.1" {
		t.Fatalf("want exactly host 127.0.0.1, got %+v", hosts)
	}
	if len(hosts[0].OpenPorts) != 1 || hosts[0].OpenPorts[0] != port {
		t.Fatalf("want open port %d, got %v", port, hosts[0].OpenPorts)
	}
	if hosts[0].ID == "" {
		t.Errorf("host ID should never be empty")
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	_, err := Run(ctx, "10.0.0.0/24", Config{Ports: []int{9}, Concurrency: 8, TimeoutMs: 200}, nil)
	if err == nil {
		t.Errorf("expected cancellation error")
	}
}
