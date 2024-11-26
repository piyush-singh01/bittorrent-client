package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

// PeerConnection represents an active connection with a peerConnection
type PeerConnection struct {
	tcpConn           net.Conn
	isActive          bool
	peerId            [20]byte
	peerIdStr         string
	piecesBitfield    *Bitset
	amChoking         bool
	amInterested      bool
	peerChoking       bool
	peerInterested    bool
	lastWriteTime     time.Time
	lastReadTime      time.Time
	writeChannel      chan *PeerMessage
	quitReaderChannel chan struct{}
	quitWriterChannel chan struct{}
	peerReaderStarted bool
	peerWriterStarted bool
}

func NewPeerConnection(peer Peer, conn net.Conn, session *TorrentSession) *PeerConnection {
	return &PeerConnection{
		tcpConn:           conn,
		peerId:            peer.PeerId,
		peerIdStr:         hex.EncodeToString(peer.PeerId[:]),
		piecesBitfield:    nil,
		amChoking:         true,
		amInterested:      false,
		peerChoking:       true,
		peerInterested:    false,
		lastWriteTime:     time.Now(),
		writeChannel:      make(chan *PeerMessage, 1),
		quitReaderChannel: make(chan struct{}, 1),
		quitWriterChannel: make(chan struct{}, 1),
		peerReaderStarted: false,
		peerWriterStarted: false,
	}
}

func NewPeerConnectionWithReaderAndWriter(peer Peer, conn net.Conn, session *TorrentSession) *PeerConnection {
	var peerConnection = &PeerConnection{
		tcpConn:           conn,
		peerId:            peer.PeerId,
		peerIdStr:         hex.EncodeToString(peer.PeerId[:]),
		piecesBitfield:    nil,
		amChoking:         true,
		amInterested:      false,
		peerChoking:       true,
		peerInterested:    false,
		lastWriteTime:     time.Now(),
		writeChannel:      make(chan *PeerMessage, 30),
		quitReaderChannel: make(chan struct{}, 1),
		quitWriterChannel: make(chan struct{}, 1),
		peerReaderStarted: false,
		peerWriterStarted: false,
	}
	err := session.AddPeerToActiveList(peerConnection)
	if err != nil {
		log.Printf("can not add peerConnection to active list: %v: discarding connection", err)
		return nil
	}
	go peerConnection.PeerReader(session)
	go peerConnection.PeerWriter(session)
	return peerConnection
}

func (pc *PeerConnection) ReadMessage(session *TorrentSession) (message *PeerMessage, n int, err error) {
	buffer := make([]byte, ConnectionBufferSize)
	n, err = pc.tcpConn.Read(buffer)
	message = ParsePeerMessage(buffer[:n])
	log.Printf("read %d bytes; message of type %d from peerConnection %s", n, message.MessageId, pc.peerIdStr)
	session.rateTracker.RecordDownload(pc.peerIdStr, n)
	pc.UpdateLastReadTime()
	return
}

func (pc *PeerConnection) ReadBytes(session *TorrentSession) (data []byte, n int, err error) {
	buffer := make([]byte, ConnectionBufferSize)
	n, err = pc.tcpConn.Read(buffer)
	log.Printf("read %d  bytes from peerConnection %s", n, pc.peerIdStr)
	data = buffer[:n]
	session.rateTracker.RecordDownload(pc.peerIdStr, n)
	pc.UpdateLastReadTime()
	return
}

func (pc *PeerConnection) WriteMessage(message *PeerMessage, session *TorrentSession) (n int, err error) {
	n, err = pc.tcpConn.Write(message.Serialize())
	log.Printf("written %d bytes; message of type %d to peerConnection %s", n, message.MessageId, pc.peerIdStr)
	pc.UpdateLastWriteTime()
	session.rateTracker.RecordUpload(pc.peerIdStr, n)
	return
}

func (pc *PeerConnection) WriteBytes(data []byte, session *TorrentSession) (n int, err error) {
	n, err = pc.tcpConn.Write(data)
	log.Printf("written %d bytes to peerConnection %s", n, pc.peerIdStr)
	pc.UpdateLastWriteTime()
	session.rateTracker.RecordUpload(pc.peerIdStr, n)
	return
}

func (pc *PeerConnection) UpdateLastWriteTime() {
	pc.lastWriteTime = time.Now()
}

func (pc *PeerConnection) UpdateLastReadTime() {
	pc.lastReadTime = time.Now()
}

func (pc *PeerConnection) StartReaderAndWriter(session *TorrentSession) {
	if pc.peerReaderStarted {
		log.Printf("reader for peerConnection %s is already started", pc.peerIdStr)
	}
	if pc.peerWriterStarted {
		log.Printf("writer for peerConnection %s is already started", pc.peerIdStr)
	}
	err := session.AddPeerToActiveList(pc)
	if err != nil {
		log.Printf("can not add peerConnection to active list: %v: discarding connection", err)
		return
	}
	go pc.PeerReader(session)
	go pc.PeerWriter(session)
}

// DialPeerWithTimeoutTCP this is meant to be run as a goroutine
func DialPeerWithTimeoutTCP(peer Peer, session *TorrentSession) (*PeerConnection, error) {
	var address *net.TCPAddr
	var err error

	switch peer.Type {
	case IPv4:
		address, err = net.ResolveTCPAddr("tcp4", peer.IP.String()+":"+strconv.Itoa(int(peer.Port)))
	case IPv6:
		address, err = net.ResolveTCPAddr("tcp6", "["+peer.IP.String()+"]"+":"+strconv.Itoa(int(peer.Port)))
	default:
		return nil, fmt.Errorf("unsupported peerConnection address type: only ipv4 and ipv6 supported")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to resolve peerConnection address : %v", err)
	}

	log.Printf("initiating tcp connection with peerConnection %s", peer.String())

	conn, err := net.DialTimeout("tcp", address.String(), session.configurable.initialTcpConnectionTimeout)
	if err != nil {
		return nil, fmt.Errorf("error initiating tcp connection with peerConnection %s: %v", hex.EncodeToString(peer.PeerId[:]), err)
	}
	peerConnection := NewPeerConnection(peer, conn, session)
	return peerConnection, nil
}

func (pc *PeerConnection) CloseConnection(session *TorrentSession) {
	if pc.peerReaderStarted {
		select {
		case pc.quitReaderChannel <- struct{}{}:
			log.Printf("quit reader signal sent to peerConnection %s", pc.peerIdStr)
		default:
			log.Printf("peerConnection %s not listening for quit reader signal", pc.peerIdStr)
		}
	}

	if pc.peerWriterStarted {
		select {
		case pc.quitWriterChannel <- struct{}{}:
			log.Printf("quit writer signal sent to peerConnection %s", pc.peerIdStr)
		default:
			log.Printf("peerConnection %s not listening for quit writer signal", pc.peerIdStr)
		}
	}

	// todo: should we fix this? we are closing the connection in dialtcp, before adding to to active list
	err := session.RemovePeerFromActiveList(pc)
	if err != nil {
		log.Printf("can not remove peerConnection %s from active list: %v", pc.peerIdStr, err)
	}

	if err = pc.tcpConn.Close(); err != nil {
		log.Printf("failed to close connection with %s: %v", pc.peerIdStr, err)
	}
}
