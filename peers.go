package main

import (
	"fmt"
	"net"
)

type IPType int

const (
	IPv4 IPType = iota
	IPv6
	InvalidIpType
)

func GetIPType(ip net.IP) IPType {
	if ip.To4() != nil {
		return IPv4
	} else if ip.To16() != nil {
		return IPv6
	}
	return InvalidIpType
}

type Peer struct {
	PeerId [20]byte
	IP   net.IP
	Type IPType
	Port uint16
}

func (p Peer) String() string {
	return fmt.Sprintf(
		"IP: %-39s | Port: %-5d | PeerId: %-40x",
		p.IP.String(),
		p.Port,
		p.PeerId,
	)
}
