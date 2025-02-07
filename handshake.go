package main

import (
	"fmt"
	"log"
	"net"
)

// HandshakeMessage struct for peerConnection handshake
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

func (hs *HandshakeMessage) serialize() []byte {
	totalLength := 1 + len(hs.Pstr) + 8 + 20 + 20
	serializedHandshake := make([]byte, totalLength)

	serializedHandshake[0] = byte(len(hs.Pstr)) // First byte is the length of protocol string
	var pos = 1
	pos += copy(serializedHandshake[pos:], hs.Pstr)         // protocol string
	pos += copy(serializedHandshake[pos:], make([]byte, 8)) // reserved 8 bytes
	pos += copy(serializedHandshake[pos:], hs.InfoHash[:])  // info-hash
	pos += copy(serializedHandshake[pos:], hs.PeerId[:])    // peerConnection id
	return serializedHandshake
}

func (hs *HandshakeMessage) validate(torrent *Torrent) error {
	if hs == nil {
		return fmt.Errorf("invalid handshake")
	}

	if hs.Pstr != protocolString {
		return fmt.Errorf("invalid protocol string identifier: %s", hs.Pstr)
	}

	if torrent.InfoHash != hs.InfoHash {
		return fmt.Errorf("invalid info-hash recieved")
	}

	return nil
}

func parseHandshake(handshake []byte) *HandshakeMessage {
	if handshake == nil || len(handshake) < 1+int(handshake[0])+8+20+20 {
		return nil
	}

	lenPstr := int(handshake[0])
	var infohash [20]byte
	var peerId [20]byte

	pstr := string(handshake[1 : lenPstr+1])
	log.Printf("parsing peer handshake: length of pstr is %d and pstr is %s", lenPstr, pstr)
	copy(infohash[:], handshake[lenPstr+9:lenPstr+29])
	copy(peerId[:], handshake[lenPstr+29:lenPstr+49])

	return &HandshakeMessage{
		Pstr:     pstr,
		InfoHash: infohash,
		PeerId:   peerId,
	}
}

func PerformHandshake(conn *PeerConnection, session *TorrentSession, peerId [20]byte) error {
	torrent := session.torrent
	handshakeMessage := NewHandshakeMessage(torrent.InfoHash, peerId)
	_, err := sendHandshake(conn, handshakeMessage, session)
	if err != nil {
		return fmt.Errorf("error sending handshake message: %v", err)
	}

	log.Printf("sent handshake to peerConnection %s", conn.peerIdStr)
	peerHandshake, err := receiveHandshake(conn, session)
	if err != nil {
		return fmt.Errorf("error receiving handshake message from peerConnection: %v", err)
	}
	log.Printf("received handshake from peerConnection %s", conn.peerIdStr)

	if err = peerHandshake.validate(torrent); err != nil {
		return fmt.Errorf("error validating received handshake from peerConnection %s: %v", conn.peerIdStr, err)
	}
	log.Print("info-hash validated")
	return nil
}

func sendHandshake(conn *PeerConnection, message *HandshakeMessage, session *TorrentSession) (n int, err error) {
	serializedHandshake := message.serialize()
	n, err = conn.WriteBytes(serializedHandshake, session)
	if err != nil {
		return 0, fmt.Errorf("error sending handshake message to peerConnection: %v", err)
	}
	return
}

func receiveHandshake(conn *PeerConnection, session *TorrentSession) (*HandshakeMessage, error) {
	buffer, n, err := conn.ReadBytes(session)
	if err != nil {
		return nil, fmt.Errorf("error receiving handshake from peerConnection: %v", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("no handshake recieved from peerConnection")
	}

	peerHandshake := parseHandshake(buffer[:n])
	if peerHandshake == nil {
		return nil, fmt.Errorf("no handshake recieved from peerConnection")
	}

	return peerHandshake, nil
}

func HandleHandshake(conn net.Conn, torrentSession *TorrentSession) (*HandshakeMessage, error) {
	receivedHandshake, err := acceptHandshake(conn)
	if err != nil {
		return nil, err
	}
	log.Printf("received handshake from incoming peerConnection")

	if err = receivedHandshake.validate(torrentSession.torrent); err != nil {
		return nil, fmt.Errorf("error validating handshake from connection: %v", err)
	}

	handshakeMessage := NewHandshakeMessage(torrentSession.torrent.InfoHash, torrentSession.localPeerId)
	_, err = respondHandshake(conn, handshakeMessage)
	if err != nil {
		return nil, err
	}
	log.Printf("sent handshake to incoming peerConnection")
	return receivedHandshake, nil
}

func acceptHandshake(conn net.Conn) (*HandshakeMessage, error) {
	buffer := make([]byte, 16384)
	n, err := conn.Read(buffer)
	log.Printf("handshake received from peerConnection")
	if err != nil {
		return nil, fmt.Errorf("error accepting handshake from peerConnection: %v", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("no handshake recieved from peerConnection")
	}

	peerHandshake := parseHandshake(buffer[:n])
	if peerHandshake == nil {
		return nil, fmt.Errorf("no handshake recieved from peerConnection")
	}
	return peerHandshake, nil
}

func respondHandshake(conn net.Conn, message *HandshakeMessage) (n int, err error) {
	serializedHandshake := message.serialize()
	n, err = conn.Write(serializedHandshake)
	if err != nil {
		return 0, fmt.Errorf("error responding to handshake by peerConnection: %v", err)
	}
	return
}
