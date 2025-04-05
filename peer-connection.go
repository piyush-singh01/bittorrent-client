package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

/** TOC
- INIT
	- New Peer Without Reader and Writer Goroutines
	- Start Reader and Writer Goroutines
	- New Peer With Reader and Writer Goroutines
	- Dial New TCP connection to create a connection
- READ
	- ReadBytes
	- ReadMessage
	- SafeUpdateLastReadTime
- WRITE
	- WriteBytes
	- WriteMessage
	- SafeUpdateLastWriteTime
- CLOSE
	- CloseConnection
*/

// PeerConnection represents an active connection with a peer
type PeerConnection struct {
	mutex    sync.Mutex
	isActive bool

	/* Immutable fields */
	tcpConn   net.Conn
	peerId    [20]byte
	peerIdStr string

	/* Mutable Fields */
	stateMutex     sync.RWMutex
	amChoking      bool
	amInterested   bool
	peerChoking    bool
	peerInterested bool

	piecesMutex    sync.RWMutex
	piecesBitfield *Bitset

	timeMutex     sync.RWMutex
	lastWriteTime time.Time
	lastReadTime  time.Time

	/* Channels */
	writeChannel chan *PeerMessage

	quitReaderChannel chan struct{}
	quitWriterChannel chan struct{}

	peerReaderStarted bool
	peerWriterStarted bool
}

/************************************** INIT **************************************/

func NewPeerConnection(peer Peer, conn net.Conn) *PeerConnection {
	peerConnection := &PeerConnection{
		isActive: false,

		tcpConn:        conn,
		piecesBitfield: nil,

		peerId:    peer.PeerId,
		peerIdStr: hex.EncodeToString(peer.PeerId[:]),

		writeChannel: make(chan *PeerMessage, 10),

		quitReaderChannel: make(chan struct{}, 1),
		quitWriterChannel: make(chan struct{}, 1),

		peerReaderStarted: false,
		peerWriterStarted: false,
	}

	if err := peerConnection.tcpConn.(*net.TCPConn).SetKeepAlive(true); err != nil {
		log.Printf("error setting keep alive for peer %s", peer.PeerId)
	}
	if err := peerConnection.tcpConn.(*net.TCPConn).SetKeepAlivePeriod(30 * time.Second); err != nil {
		log.Printf("error setting keep alive period for peer %s", peer.PeerId)
	}

	peerConnection.amChoking = true
	peerConnection.amInterested = false
	peerConnection.peerChoking = true
	peerConnection.peerInterested = false
	return peerConnection
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
	session.InitializePeer(pc)
}

func CreatePeerConnectionAndStartReaderWriter(peer Peer, conn net.Conn, session *TorrentSession) {
	var peerConnection = NewPeerConnection(peer, conn)
	peerConnection.StartReaderAndWriter(session)
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
	pc.SafeUpdateLastReadTime()
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
	pc.SafeUpdateLastReadTime()
	return
}

func (pc *PeerConnection) SafeUpdateLastReadTime() {
	pc.timeMutex.Lock()
	defer pc.timeMutex.Unlock()
	pc.lastReadTime = time.Now()
}

/****************************** WRITE TO PEER ******************************/

func (pc *PeerConnection) WriteMessage(message *PeerMessage, rateTracker *RateTracker) (n int, err error) {
	n, err = pc.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, err
	}

	log.Printf("written %d bytes; message of type %d to peer %s", n, message.MessageId, pc.peerIdStr)
	pc.SafeUpdateLastWriteTime()
	rateTracker.RecordUpload(pc.peerIdStr, n)
	return
}

func (pc *PeerConnection) WriteBytes(data []byte, rateTracker *RateTracker) (n int, err error) {
	n, err = pc.tcpConn.Write(data)
	if err != nil {
		return 0, err
	}

	log.Printf("written %d bytes to peer %s", n, pc.peerIdStr)
	pc.SafeUpdateLastWriteTime()
	rateTracker.RecordUpload(pc.peerIdStr, n)
	return
}

func (pc *PeerConnection) SafeUpdateLastWriteTime() {
	pc.timeMutex.Lock()
	defer pc.timeMutex.Unlock()
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
