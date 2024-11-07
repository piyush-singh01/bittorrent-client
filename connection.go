package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

const LocalPort uint16 = 7777

// PeerConnection represents an active connection with a peer
type PeerConnection struct {
	tcpConn net.Conn
}

// DialPeerWithTimeoutTCP this is meant to be run as a goroutine
func DialPeerWithTimeoutTCP(peer Peer, timeout time.Duration) (*PeerConnection, error) {
	var address *net.TCPAddr
	var err error

	if peer.Type == IPv4 {
		address, err = net.ResolveTCPAddr("tcp4", peer.IP.String()+":"+strconv.Itoa(int(peer.Port)))
	} else if peer.Type == IPv6 {
		address, err = net.ResolveTCPAddr("tcp6", "["+peer.IP.String()+"]"+":"+strconv.Itoa(int(peer.Port)))
	} else {
		return nil, fmt.Errorf("unsupported peer address type: only ipv4 and ipv6 supported")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to resolve peer address : %v", err)
	}

	log.Printf("initiating tcp connection with peer %s", peer.String())

	conn, err := net.DialTimeout("tcp", address.String(), timeout)
	if err != nil {
		return nil, fmt.Errorf("error intiating tcp connection with peer %s from local port %d: %v", peer.String(), LocalPort, err)
	}
	peerConnection := &PeerConnection{tcpConn: conn}
	return peerConnection, nil
}

func (pc *PeerConnection) CloseConnection() {
	if err := pc.tcpConn.Close(); err != nil {
		log.Printf("failed to close connection: %v", err)
	}
}
