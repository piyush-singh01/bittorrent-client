package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

/* TOC
- INIT
	- New Peer Without Reader and Writer Goroutines
	- Start Reader and Writer Goroutines
	- New Peer With Reader and Writer Goroutines
	- Dial New TCP connection to create a connection
- READ
	- ReadBytes
	- ReadMessage
	- UpdateLastReadTime
- WRITE
	- WriteBytes
	- WriteMessage
	- UpdateLastWriteTime
- SEND
	-
- CLOSE
	- CloseConnection
*/

// PeerConnection represents an active connection with a peerConnection
type PeerConnection struct {
	tcpConn        net.Conn
	isActive       bool
	piecesBitfield *Bitset

	peerId    [20]byte
	peerIdStr string

	amChoking      bool
	amInterested   bool
	peerChoking    bool
	peerInterested bool

	lastWriteTime time.Time
	lastReadTime  time.Time

	writeChannel chan *PeerMessage

	quitReaderChannel chan struct{}
	quitWriterChannel chan struct{}

	peerReaderStarted bool
	peerWriterStarted bool
}

/************************************** INIT **************************************/

func NewPeerConnection(peer Peer, conn net.Conn) *PeerConnection {
	return &PeerConnection{
		tcpConn:        conn,
		isActive:       false,
		piecesBitfield: nil,

		peerId:    peer.PeerId,
		peerIdStr: hex.EncodeToString(peer.PeerId[:]),

		amChoking:      true,
		amInterested:   false,
		peerChoking:    true,
		peerInterested: false,

		writeChannel: make(chan *PeerMessage, 10),

		quitReaderChannel: make(chan struct{}, 1),
		quitWriterChannel: make(chan struct{}, 1),

		peerReaderStarted: false,
		peerWriterStarted: false,
	}
}

func (pc *PeerConnection) StartReaderAndWriter(session *TorrentSession) {
	if pc.peerReaderStarted {
		log.Printf("reader for peer %s is already started", pc.peerIdStr)
	}
	if pc.peerWriterStarted {
		log.Printf("writer for peer %s is already started", pc.peerIdStr)
	}
	go pc.PeerReader(session)
	go pc.PeerWriter(session)
}

func NewPeerConnectionWithReaderAndWriter(peer Peer, conn net.Conn, session *TorrentSession) *PeerConnection {
	var peerConnection = &PeerConnection{
		tcpConn:        conn,
		isActive:       false, // remove for now,
		piecesBitfield: nil,

		peerId:    peer.PeerId,
		peerIdStr: hex.EncodeToString(peer.PeerId[:]),

		amChoking:      true,
		amInterested:   false,
		peerChoking:    true,
		peerInterested: false,

		lastWriteTime: time.Now(),

		writeChannel: make(chan *PeerMessage, 30),

		quitReaderChannel: make(chan struct{}, 1),
		quitWriterChannel: make(chan struct{}, 1),

		peerReaderStarted: false,
		peerWriterStarted: false,
	}
	go peerConnection.PeerReader(session)
	go peerConnection.PeerWriter(session)
	return peerConnection
}

func DialPeerWithTimeoutTCP(peer Peer, session *TorrentSession) (*PeerConnection, error) {
	var address *net.TCPAddr
	var err error

	switch peer.Type {
	case IPv4:
		address, err = net.ResolveTCPAddr("tcp4", peer.IP.String()+":"+strconv.Itoa(int(peer.Port)))
	case IPv6:
		address, err = net.ResolveTCPAddr("tcp6", "["+peer.IP.String()+"]"+":"+strconv.Itoa(int(peer.Port)))
	default:
		return nil, fmt.Errorf("unsupported peer address type: only ipv4 and ipv6 supported")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to resolve peer address : %v", err)
	}

	log.Printf("initiating tcp connection with peer %s", peer.String())

	conn, err := net.DialTimeout("tcp", address.String(), session.configurable.tcpDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("error initiating tcp connection with peer %s: %v", hex.EncodeToString(peer.PeerId[:]), err)
	}
	peerConnection := NewPeerConnection(peer, conn)
	return peerConnection, nil
}

/****************************** READ FROM PEER ******************************/

func (pc *PeerConnection) ReadMessage(rateTracker *RateTracker) (message *PeerMessage, n int, err error) {
	buffer := make([]byte, ConnectionBufferSize)
	n, err = pc.tcpConn.Read(buffer)
	if err != nil {
		return nil, 0, err
	}

	message, err = ParsePeerMessage(buffer[:n])
	if err != nil {
		return nil, 0, err
	}
	log.Printf("read %d bytes; message of type %d from peer %s", n, message.MessageId, pc.peerIdStr)
	rateTracker.RecordDownload(pc.peerIdStr, n)
	pc.UpdateLastReadTime()
	return
}

func (pc *PeerConnection) ReadBytes(rateTracker *RateTracker) (data []byte, n int, err error) {
	buffer := make([]byte, ConnectionBufferSize)
	n, err = pc.tcpConn.Read(buffer)
	if err != nil {
		return nil, 0, err
	}

	log.Printf("read %d  bytes from peer %s", n, pc.peerIdStr)
	data = buffer[:n]
	rateTracker.RecordDownload(pc.peerIdStr, n)
	pc.UpdateLastReadTime()
	return
}

func (pc *PeerConnection) UpdateLastReadTime() {
	pc.lastReadTime = time.Now()
}

/****************************** WRITE TO PEER ******************************/

func (pc *PeerConnection) WriteMessage(message *PeerMessage, rateTracker *RateTracker) (n int, err error) {
	n, err = pc.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, err
	}

	log.Printf("written %d bytes; message of type %d to peer %s", n, message.MessageId, pc.peerIdStr)
	pc.UpdateLastWriteTime()
	rateTracker.RecordUpload(pc.peerIdStr, n)
	return
}

func (pc *PeerConnection) WriteBytes(data []byte, rateTracker *RateTracker) (n int, err error) {
	n, err = pc.tcpConn.Write(data)
	if err != nil {
		return 0, err
	}

	log.Printf("written %d bytes to peer %s", n, pc.peerIdStr)
	pc.UpdateLastWriteTime()
	rateTracker.RecordUpload(pc.peerIdStr, n)
	return
}

func (pc *PeerConnection) UpdateLastWriteTime() {
	pc.lastWriteTime = time.Now()
}

/****************************** CLOSE CONNECTION ******************************/

func (pc *PeerConnection) CloseConnection() {
	if pc.peerReaderStarted {
		select {
		case pc.quitReaderChannel <- struct{}{}:
			log.Printf("quit reader signal sent to peer %s", pc.peerIdStr)
		default:
			log.Printf("peer %s not listening for quit reader signal", pc.peerIdStr)
		}
	}

	if pc.peerWriterStarted {
		select {
		case pc.quitWriterChannel <- struct{}{}:
			log.Printf("quit writer signal sent to peer %s", pc.peerIdStr)
		default:
			log.Printf("peer %s not listening for quit writer signal", pc.peerIdStr)
		}
	}

	if err := pc.tcpConn.Close(); err != nil {
		log.Printf("failed to close connection with %s: %v", pc.peerIdStr, err)
	}
}
