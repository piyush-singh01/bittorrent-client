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

type TrackerResponse struct {
	Peers      []Peer
	Interval   uint32
	Incomplete int
	Complete   int
}

func NewEmptyTrackerResponse() *TrackerResponse {
	return &TrackerResponse{}
}

func (tr *TrackerResponse) String() string {
	peerListStr := ""
	for _, peer := range tr.Peers {
		peerListStr += fmt.Sprintln(peer)
	}
	return fmt.Sprintf(
		"TrackerResponse:\n"+
			"- Interval: %d seconds\n"+
			"- Incomplete: %d\n"+
			"- Complete: %d\n"+
			"- Peers Count: %d\n"+
			"- Peers: \n%s",
		tr.Interval, tr.Incomplete, tr.Complete, len(tr.Peers), peerListStr,
	)
}
