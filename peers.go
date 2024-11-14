package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
)

// generateLocalPeerId generates a Peer ID for the client.
func generateLocalPeerId() ([20]byte, error) {
	var localPeerId [20]byte

	prefix := "-PTC001-"
	copy(localPeerId[:], prefix)

	randomBytes := make([]byte, 13)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return localPeerId, fmt.Errorf("failed to generate random bytes: %v", err)
	}
	copy(localPeerId[7:], randomBytes)

	return localPeerId, nil
}

type IPType int

const (
	IPv4 IPType = iota
	IPv6
	Invalid
)

func GetIPType(ip net.IP) IPType {
	if ip.To4() != nil {
		return IPv4
	} else if ip.To16() != nil {
		return IPv6
	}
	return Invalid
}

type Peer struct {
	PeerId [20]byte
	IP     net.IP
	Type   IPType
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

type PeerState struct {
	conn           net.Conn
	peerId         [20]byte
	peerIdStr      string
	piecesBitfield *Bitset
	amChoking      bool
	amInterested   bool
	peerChoking    bool
	peerInterested bool
}

func NewPeer(peer Peer, conn net.Conn) *PeerState {
	return &PeerState{
		conn:           conn,
		peerId:         peer.PeerId,
		peerIdStr:      hex.EncodeToString(peer.PeerId[:]),
		piecesBitfield: nil,
		amChoking:      true,
		amInterested:   false,
		peerChoking:    true,
		peerInterested: false,
	}
}
