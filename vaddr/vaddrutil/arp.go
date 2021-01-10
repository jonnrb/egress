package vaddrutil

import (
	"fmt"
	"net"

	"github.com/mdlayher/arp"
	"go.jonnrb.io/egress/fw"
)

type GratuitousARP struct {
	IP     net.IP
	HWAddr net.HardwareAddr
	Link   fw.Link
}

// Sends out a gratuitous ARP to speed up failover resolution.
func (a *GratuitousARP) Start() error {
	i, err := net.InterfaceByName(a.Link.Name())
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: could not get interface %q: %w", a.Link.Name(), err)
	}
	c, err := arp.Dial(i)
	if err != nil {
		return fmt.Errorf("vaddrutil: could not get ARP conn: %w", err)
	}
	// Construct a gratuitous ARP request.
	p, err := arp.NewPacket(
		arp.OperationRequest, a.HWAddr, a.IP, broadcastHWAddr, a.IP)
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: could not construct gratuitous ARP request: %w", err)
	}
	err = c.WriteTo(p, broadcastHWAddr)
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: could not write gratuitous ARP request: %w", err)
	}
	return nil
}

var broadcastHWAddr = net.HardwareAddr{255, 255, 255, 255, 255, 255}

// Does nothing since we only gratuitous ARP when bringing up a vaddr.
func (a *GratuitousARP) Stop() error {
	return nil
}
