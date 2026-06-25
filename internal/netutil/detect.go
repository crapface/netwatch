// Package netutil handles adapter detection and ARP-based MAC lookup.
package netutil

import (
	"fmt"
	"net"
)

// Adapter describes a candidate network adapter.
type Adapter struct {
	Name string
	IP   net.IP
	CIDR string
}

// preferredIP returns the local IPv4 the OS would use for outbound traffic.
// A UDP "connect" sets the socket's local address without sending any packets,
// which reliably identifies the active adapter without admin rights.
func preferredIP() net.IP {
	c, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer c.Close()
	if ua, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return ua.IP.To4()
	}
	return nil
}

// ListAdapters returns all up, non-loopback IPv4 adapters with their CIDR.
func ListAdapters() []Adapter {
	var out []Adapter
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifi.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			network := ipnet.IP.Mask(ipnet.Mask)
			ones, _ := ipnet.Mask.Size()
			out = append(out, Adapter{
				Name: ifi.Name,
				IP:   ip4,
				CIDR: fmt.Sprintf("%s/%d", network.String(), ones),
			})
		}
	}
	return out
}

// DetectCIDR auto-detects the active adapter's IPv4 subnet as a CIDR string,
// e.g. "192.168.1.0/24", and returns the adapter name. It prefers the adapter
// that owns the OS's outbound IP, then falls back to the first private adapter.
func DetectCIDR() (cidr string, iface string, err error) {
	pref := preferredIP()
	adapters := ListAdapters()
	if len(adapters) == 0 {
		return "", "", fmt.Errorf("no active IPv4 adapter found")
	}
	var firstPrivate, first *Adapter
	for i := range adapters {
		a := &adapters[i]
		if pref != nil && a.IP.Equal(pref) {
			return a.CIDR, a.Name, nil
		}
		if first == nil {
			first = a
		}
		if firstPrivate == nil && a.IP.IsPrivate() {
			firstPrivate = a
		}
	}
	if firstPrivate != nil {
		return firstPrivate.CIDR, firstPrivate.Name, nil
	}
	return first.CIDR, first.Name, nil
}
