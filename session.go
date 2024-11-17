package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

// Configurable todo, club all tracker stuff into another tracker struct?
type Configurable struct {
	trackerHttpRequestTimeout   time.Duration // the timeout till when tracker request is fulfilled, taking in account the exponential backoff
	trackerQueryTimeout         time.Duration // the timeout for http request to the tracker
	trackerResponseMinPeers     int
	initialTcpConnectionTimeout time.Duration
	listenerPort                uint16
}

type TorrentSession struct {
	torrent        *Torrent
	localPeerId    [20]byte
	configurable   *Configurable
	peerConnection map[string]*PeerConnection
	bitfield       *Bitset
	//connectionChannel chan *PeerConnection // todo: discard this and use the two read and write channels as below
	readChannel  chan *PeerConnection
	writeChannel chan *PeerConnection
	listener     *Listener
}

func NewTorrentSession(torrent *Torrent, localPeerId [20]byte) (*TorrentSession, error) {
	configurable := &Configurable{
		trackerHttpRequestTimeout:   time.Second * 4,
		trackerQueryTimeout:         time.Second * 36,
		initialTcpConnectionTimeout: time.Second * 5,
		trackerResponseMinPeers:     4,
		listenerPort:                8888,
	}
	return &TorrentSession{
		torrent:        torrent,
		localPeerId:    localPeerId,
		configurable:   configurable,
		peerConnection: make(map[string]*PeerConnection),
		bitfield:       NewBitset(torrent.Info.NumPieces),
		//connectionChannel: make(chan *PeerConnection, 50),
		readChannel:  make(chan *PeerConnection, 50),
		writeChannel: make(chan *PeerConnection, 50),
		listener:     nil,
	}, nil
}

func (ts *TorrentSession) MountListenerOnDefaultPort() error {
	listener, err := NewListener(ts.configurable.listenerPort)
	if err != nil {
		return err
	}
	ts.listener = listener
	return nil
}

// StartListening Meant to be run as a goroutine
func (ts *TorrentSession) StartListening() error {
	if ts.listener == nil {
		return fmt.Errorf("no listener found in torrent session")
	}

	l := ts.listener
	for {
		var conn net.Conn
		conn, err := l.conn.Accept()
		if err != nil {
			log.Printf("listener accept failed")
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

		receivedHandshake, err := HandleHandshake(conn, ts)
		if err != nil {
			log.Printf("error performing handshake with incoming peer")
			continue
		}

		peer := Peer{
			PeerId: receivedHandshake.PeerId,
			IP:     ip,
			Type:   GetIPType(ip),
			Port:   uint16(port),
		}

		ts.readChannel <- NewPeerConnection(peer, conn)
	}
}

func (ts *TorrentSession) CloseListener() {
	err := ts.listener.conn.Close()
	if err != nil {
		log.Printf("error closing tcp listener: %v", err)
		return
	}
	log.Printf("listener closed")
}

func (ts *TorrentSession) AddPeerToActiveList(peer *PeerConnection) error {
	peerIdStr := hex.EncodeToString(peer.peerId[:])
	if _, ok := ts.peerConnection[peerIdStr]; ok {
		return fmt.Errorf("peer already in active peer list")
	}
	ts.peerConnection[peerIdStr] = peer
	return nil
}

func (ts *TorrentSession) RemovePeerFromActiveList(peer *PeerConnection) error {
	if _, ok := ts.peerConnection[peer.peerIdStr]; !ok {
		return fmt.Errorf("error removing: peer not present in active peer list")
	}
	delete(ts.peerConnection, peer.peerIdStr)
	return nil
}

func (ts *TorrentSession) SendKeepAlive(peer *PeerConnection) (n int, err error) {
	message := NewKeepAliveMessage()
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'keep-alive' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendChoke(peer *PeerConnection) (n int, err error) {
	message := NewChokeMessage()
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'choke' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendUnchoke(peer *PeerConnection) (n int, err error) {
	message := NewUnchokeMessage()
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'unchoke' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendInterested(peer *PeerConnection) (n int, err error) {
	message := NewInterestedMessage()
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'interested' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendNotInterested(peer *PeerConnection) (n int, err error) {
	message := NewNotInterestedMessage()
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'not-interested' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendHave(pieceIndex uint32, peer *PeerConnection) (n int, err error) {
	message := NewHaveMessage(pieceIndex)
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'have' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendBitfield(peer *PeerConnection) (n int, err error) {
	message := NewBitfieldMessage(ts.bitfield)
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'bitfield' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendRequest(index uint32, begin uint32, length uint32, peer *PeerConnection) (n int, err error) {
	message := NewRequestMessage(index, begin, length)
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'request' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendPiece(index uint32, begin uint32, block []byte, peer *PeerConnection) (n int, err error) {
	message := NewPieceMessage(index, begin, block)
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'piece' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendCancel(index uint32, begin uint32, length uint32, peer *PeerConnection) (n int, err error) {
	message := NewCancelMessage(index, begin, length)
	n, err = peer.tcpConn.Write(message.Serialize())
	if err != nil {
		return 0, fmt.Errorf("error sending 'cancel' message to peer %s: ", peer.peerIdStr)
	}
	return
}
