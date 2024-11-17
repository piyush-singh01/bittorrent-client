package main

import (
	"fmt"
	"net"
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
