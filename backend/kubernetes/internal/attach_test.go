package internal

import (
	"strings"
	"testing"
)

func TestDecodeNetworkStatus_fieldsAreMapped(t *testing.T) {
	const example = `
        [{
            "name": "cluster-node0",
            "interface": "eth0",
            "ips": [
                "10.254.0.18"
            ],
            "mac": "1e:0a:b0:bc:8a:ab",
            "default": true,
            "dns": {}
        }]`

	nets, err := decodeNetworkStatus(strings.NewReader(example))
	if err != nil {
		t.Fatalf("Could not decode network status: %v", err)
	}

	net := nets[0]
	if net.Name != "cluster-node0" {
		t.Errorf("'name' field not captured")
	}
	if net.Interface != "eth0" {
		t.Errorf("'interface' field not captured")
	}
	if net.IPs[0] != "10.254.0.18" {
		t.Errorf("'ips' field not captured")
	}
	if net.MAC != "1e:0a:b0:bc:8a:ab" {
		t.Errorf("'mac' field not captured")
	}
	if !net.Default {
		t.Errorf("'default' field not captured")
	}
}

func TestDecodeNetworkStatus_allowsAbsentFields(t *testing.T) {
	const example = `
        [{
            "name": "cluster-node0",
            "interface": "eth0",
            "ips": [
                "10.254.0.18"
            ],
            "mac": "1e:0a:b0:bc:8a:ab"
        }]`

	_, err := decodeNetworkStatus(strings.NewReader(example))
	if err != nil {
		t.Fatalf("Could not decode network status: %v", err)
	}
}
