package internal

import (
	"bytes"
	"net"
	"testing"
)

const exampleCNIDef = `
    {
      "cniVersion": "0.3.0",
      "type": "bridge",
      "bridge": "example",
      "ipam": {
        "type": "host-local",
        "ranges": [ [ {
          "subnet": "10.0.0.0/24",
          "rangeStart": "10.0.0.100",
          "rangeEnd": "10.0.0.200",
          "gateway": "10.0.0.1"
        } ] ]
      }
    }`

func TestExtractRanges(t *testing.T) {
	r, err := extractRanges([]byte(exampleCNIDef))
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 1 {
		t.Fatal("Expected 1 range")
	}

	expectedGW := net.IPv4(10, 0, 0, 1)
	if !r[0].Gateway.Equal(expectedGW) {
		t.Errorf("Expected gw %q; got %q", expectedGW, r[0].Gateway)
	}

	expectedSubnet := net.IPNet{
		IP:   net.IPv4(10, 0, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}
	if !r[0].Subnet.IP.Equal(expectedSubnet.IP) {
		t.Errorf("Expected subnet IP %q; got %q", expectedSubnet.IP, r[0].Subnet.IP)
	}
	if !bytes.Equal(r[0].Subnet.Mask, expectedSubnet.Mask) {
		t.Errorf("Expected subnet mask %q; got %q", expectedSubnet.Mask, r[0].Subnet.Mask)
	}
}

func TestSplitFullName_noNamespace(t *testing.T) {
	ns, n := splitFullName("foo")
	if ns != "" {
		t.Errorf("expected namespace %q; got %q", "", ns)
	}
	if n != "foo" {
		t.Errorf("expected name %q; got %q", "foo", n)
	}
}

func TestSplitFullName_withNamespace(t *testing.T) {
	ns, n := splitFullName("bar/foo")
	if ns != "bar" {
		t.Errorf("expected namespace %q; got %q", "bar", ns)
	}
	if n != "foo" {
		t.Errorf("expected name %q; got %q", "foo", n)
	}
}
