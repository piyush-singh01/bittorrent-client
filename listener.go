package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
)

type Listener struct {
	conn *net.TCPListener
	port uint16
}

func NewListener(port uint16) (*Listener, error) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(port)})
	if err != nil {
		return nil, fmt.Errorf("error mounting a listener on port %d: %v", port, err)
	}

	return &Listener{conn: listener, port: port}, nil
}

func CreateAndMountListener(session *TorrentSession) (*Listener, error) {
	listener, err := NewListener(session.configurable.listenerPort)
	if err != nil {
		return nil, err
	}
	session.listener = listener
	return listener, nil
}

// StartListening Meant to be run as a goroutine
func (l *Listener) StartListening(session *TorrentSession) error {
	for {
		var conn net.Conn
		conn, err := l.conn.Accept()
		if err != nil {
			log.Printf("listener accept failed: %v", err)
			continue
		}

		remoteAddr := conn.RemoteAddr().String()
		host, portStr, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			log.Printf("error splitting host and port for connection")
			continue
		}

		ip := net.ParseIP(host)
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Printf("error converting port-string to integer: %v", err)
			continue
		}

		receivedHandshake, err := HandleHandshake(conn, session)
		if err != nil {
			log.Printf("can not perform handshake with incoming peer: %v", err)
			continue
		}
		log.Printf("handshake successful with peer %s", receivedHandshake.PeerId)
		peer := Peer{
			PeerId: receivedHandshake.PeerId,
			IP:     ip,
			Type:   GetIPType(ip),
			Port:   uint16(port),
		}

		NewPeerConnectionWithReaderAndWriter(peer, conn, session)
		log.Printf("peer connection created with reader and writer goroutines, with peer %s", receivedHandshake.PeerId)
	}
}

func (l *Listener) CloseListener() {
	err := l.conn.Close()
	if err != nil {
		log.Printf("error closing tcp listener: %v", err)
		return
	}
	log.Printf("listener closed")
}
