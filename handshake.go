package main

import (
	"fmt"
	"log"
	"net"
)

// HandshakeMessage struct for peer handshake
type HandshakeMessage struct {
	Pstr     string
	InfoHash [20]byte
	PeerId   [20]byte
}

const protocolString = "BitTorrent protocol"

func NewHandshakeMessage(infoHash [20]byte, peerId [20]byte) *HandshakeMessage {
	return &HandshakeMessage{
		Pstr:     protocolString,
		InfoHash: infoHash,
		PeerId:   peerId,
	}
}

func (hs *HandshakeMessage) String() string {
	return fmt.Sprintf("Protocol String: %s | InfoHash: %x | PeerId: %x",
		hs.Pstr,
		hs.InfoHash[:],
		hs.PeerId[:])
}

func (hs *HandshakeMessage) Serialize() []byte {
	totalLength := 1 + len(hs.Pstr) + 8 + 20 + 20
	serializedHandshake := make([]byte, totalLength)

	serializedHandshake[0] = byte(len(hs.Pstr)) // First byte is the length of protocol string
	var pos = 1
	pos += copy(serializedHandshake[pos:], hs.Pstr)         // protocol string
	pos += copy(serializedHandshake[pos:], make([]byte, 8)) // reserved 8 bytes
	pos += copy(serializedHandshake[pos:], hs.InfoHash[:])  // info-hash
	pos += copy(serializedHandshake[pos:], hs.PeerId[:])    // peer id
	return serializedHandshake
}

func parseHandshake(handshake []byte) *HandshakeMessage {
	if handshake == nil || len(handshake) < 1+int(handshake[0])+8+20+20 {
		return nil
	}

	lenPstr := int(handshake[0])
	var infohash [20]byte
	var peerId [20]byte

	pstr := string(handshake[1 : lenPstr+1])
	copy(infohash[:], handshake[lenPstr+9:lenPstr+29])
	copy(peerId[:], handshake[lenPstr+29:lenPstr+49])

	return &HandshakeMessage{
		Pstr:     pstr,
		InfoHash: infohash,
		PeerId:   peerId,
	}
}

func PerformHandshake(conn net.Conn, torrent *Torrent, peerId [20]byte) error {
	handshakeMessage := NewHandshakeMessage(torrent.InfoHash, peerId)
	_, err := sendHandshake(conn, handshakeMessage)
	if err != nil {
		return fmt.Errorf("error sending handshake message: %v", err)
	}

	log.Printf("sent data to peer")
	peerHandshake, err := receiveHandshake(conn)
	if err != nil {
		return fmt.Errorf("error receiving handshake message from peer: %v", err)
	}
	log.Printf("received data from peer")

	if torrent.InfoHash != peerHandshake.InfoHash {
		return fmt.Errorf("invalid info-hash recieved")
	}
	log.Print("info-hash validated")
	//CloseConnectionWithLog(conn) // do not close the connection after performing handshake
	//log.Print("connection closed")
	return nil
}

func sendHandshake(conn net.Conn, handshake *HandshakeMessage) (n int, err error) {
	serializedHandshake := handshake.Serialize()
	n, err = conn.Write(serializedHandshake)
	if err != nil {
		return 0, fmt.Errorf("error sending handshake to peer: %v", err)
	}
	return
}

func receiveHandshake(conn net.Conn) (*HandshakeMessage, error) {
	buffer := make([]byte, 2048)
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("error receiving handshake from peer: %v", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("no handshake recieved from peer")
	}

	peerHandshake := parseHandshake(buffer[:n])
	if peerHandshake == nil {
		return nil, fmt.Errorf("no handshake recieved from peer")
	}

	return peerHandshake, nil
}
