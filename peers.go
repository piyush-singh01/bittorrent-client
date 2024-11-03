package main

import (
	"fmt"
	"net"
)

type Peer struct {
	PeerId [20]byte
	IP     net.IP
	Port   uint16
}

func (p Peer) String() string {
	return fmt.Sprintf(
		"IP: %-39s | Port: %-5d | PeerId: %-40x",
		p.IP.String(),
		p.Port,
		p.PeerId,
	)
}

// for testing only
func printPeerList(peers []Peer) {
	for _, peer := range peers {
		fmt.Println(peer)
	}
}
