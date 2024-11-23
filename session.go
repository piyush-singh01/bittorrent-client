package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

// Configurable todo, club all tracker stuff into another tracker struct?
type Configurable struct {
	trackerHttpRequestTimeout   time.Duration // the timeout till when tracker request is fulfilled, taking in account the exponential backoff
	trackerQueryTimeout         time.Duration // the timeout for http request to the tracker
	trackerResponseMinPeers     int
	initialTcpConnectionTimeout time.Duration
	listenerPort                uint16
	trackerPollInterval         time.Duration
	keepAliveTickInterval       time.Duration
	rateTrackerTickerInterval   time.Duration
}

type TorrentSession struct {
	torrent            *Torrent
	muInterestedPeers  sync.RWMutex
	muPeerConnection   sync.RWMutex
	localPeerId        [20]byte
	configurable       *Configurable
	peerConnection     map[string]*PeerConnection
	rateTracker        *RateTracker
	bitfield           *Bitset
	quitChannel        chan *PeerConnection
	listener           *Listener
	trackerPollTicker  *time.Ticker
	keepAliveTicker    *time.Ticker
	interestedPeers    map[string]*PeerConnection
	peerRequests       map[string]*BlockRequest
	ourRequests        map[string]*BlockRequest
	peerMessageChannel chan *PeerConnectionMessage
	maxUploadSlots     int // max number of unchoked peers at a time
	rateTrackerTicker  *time.Ticker
}

func NewTorrentSession(torrent *Torrent, localPeerId [20]byte) (*TorrentSession, error) {
	configurable := &Configurable{
		trackerHttpRequestTimeout:   time.Second * 4,
		trackerQueryTimeout:         time.Second * 36,
		initialTcpConnectionTimeout: time.Second * 5,
		trackerResponseMinPeers:     4,
		listenerPort:                8888,
		keepAliveTickInterval:       time.Second * 120,
		rateTrackerTickerInterval:   time.Second,
	}
	return &TorrentSession{
		torrent:            torrent,
		localPeerId:        localPeerId,
		configurable:       configurable,
		peerConnection:     make(map[string]*PeerConnection),
		rateTracker:        NewRateTracker(),
		bitfield:           NewBitset(torrent.Info.NumPieces),
		quitChannel:        make(chan *PeerConnection, 10),
		listener:           nil,
		peerRequests:       make(map[string]*BlockRequest),
		ourRequests:        make(map[string]*BlockRequest),
		peerMessageChannel: make(chan *PeerConnectionMessage),
		maxUploadSlots:     5,
	}, nil
}

func (ts *TorrentSession) SetTrackerPolling(trackerPollInterval time.Duration) {
	if ts.trackerPollTicker != nil {
		ts.trackerPollTicker.Stop()
	}

	ts.configurable.trackerPollInterval = trackerPollInterval
	ts.trackerPollTicker = time.NewTicker(trackerPollInterval)
}

func (ts *TorrentSession) StopTrackerPolling() {
	if ts.trackerPollTicker == nil {
		log.Printf("tracker polling is already stopped")
	}
	ts.trackerPollTicker.Stop()
}

func (ts *TorrentSession) SetRateTrackerTicker() {
	if ts.rateTrackerTicker != nil {
		ts.rateTrackerTicker.Stop()
	}

	ts.rateTrackerTicker = time.NewTicker(ts.configurable.rateTrackerTickerInterval)
}

func (ts *TorrentSession) StopRateTrackerTicker() {
	if ts.rateTrackerTicker == nil {
		log.Printf("rate tracker ticker is already stopped")
		return
	}
	ts.rateTrackerTicker.Stop()
}

// StartQuitter Meant to run as a goroutine
func (ts *TorrentSession) StartQuitter() {
	for {
		connection := <-ts.quitChannel
		connection.CloseConnection(ts)
	}
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

	for {
		var conn net.Conn
		conn, err := ts.listener.conn.Accept()
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

		receivedHandshake, err := HandleHandshake(conn, ts)
		if err != nil {
			log.Printf("can not perform handshake with incoming peer: %v", err)
			continue
		}
		log.Printf("handshake successfull with peer %s", receivedHandshake.PeerId)
		peer := Peer{
			PeerId: receivedHandshake.PeerId,
			IP:     ip,
			Type:   GetIPType(ip),
			Port:   uint16(port),
		}

		NewPeerConnectionWithReaderAndWriter(peer, conn, ts)
		log.Printf("peer connection created with reader and writer goroutines, with peer %s", receivedHandshake.PeerId)
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

func (ts *TorrentSession) AddPeerToInterestedList(peer *PeerConnection) error {
	ts.muInterestedPeers.Lock()
	defer ts.muInterestedPeers.Unlock()

	if _, ok := ts.interestedPeers[peer.peerIdStr]; ok {
		return fmt.Errorf("peer already in interseted peer list")
	}
	ts.interestedPeers[peer.peerIdStr] = peer
	return nil
}

func (ts *TorrentSession) RemovePeerFromInterestedList(peer *PeerConnection) error {
	ts.muInterestedPeers.Lock()
	defer ts.muInterestedPeers.Unlock()

	if _, ok := ts.interestedPeers[peer.peerIdStr]; !ok {
		return fmt.Errorf("peer not in interseted peer list")
	}
	delete(ts.interestedPeers, peer.peerIdStr)
	return nil
}

func (ts *TorrentSession) CheckPeerInInterestedList(peer *PeerConnection) bool {
	ts.muInterestedPeers.RLock()
	defer ts.muInterestedPeers.RUnlock()

	_, ok := ts.interestedPeers[peer.peerIdStr]
	return ok
}

func (ts *TorrentSession) AddPeerToActiveList(peer *PeerConnection) error {
	ts.muPeerConnection.Lock()
	defer ts.muPeerConnection.Unlock()

	peer.isActive = true
	if _, ok := ts.peerConnection[peer.peerIdStr]; ok {
		return fmt.Errorf("peer already in active peer list")
	}
	ts.peerConnection[peer.peerIdStr] = peer
	return nil
}

func (ts *TorrentSession) RemovePeerFromActiveList(peer *PeerConnection) error {
	ts.muPeerConnection.Lock()
	defer ts.muPeerConnection.Unlock()

	peer.isActive = false
	if _, ok := ts.peerConnection[peer.peerIdStr]; !ok {
		return fmt.Errorf("error removing: peer not present in active peer list")
	}
	delete(ts.peerConnection, peer.peerIdStr)
	ts.rateTracker.RemoveConnection(peer.peerIdStr)
	return nil
}

// CheckPeerInActiveList redundant, not needed
func (ts *TorrentSession) CheckPeerInActiveList(peer *PeerConnection) bool {
	ts.muPeerConnection.RLock()
	defer ts.muPeerConnection.RUnlock()

	return peer.isActive
}

//func (ts *TorrentSession) AddPeerRequest(peer *PeerConnection, b *BlockRequest) error {
//	ts.mu.Lock()
//	defer ts.mu.Unlock()
//
//	if _, ok := ts.peerRequests[peer.peerIdStr]; ok {
//		return fmt.Errorf("block already requested by this peer %s", peer.peerIdStr)
//	}
//	ts.peerRequests[peer.peerIdStr] = b
//	return nil
//}
//
//func (ts *TorrentSession) AddOurRequest(peer *PeerConnection, b *BlockRequest) error {
//	ts.mu.Lock()
//	defer ts.mu.Unlock()
//
//	if _, ok := ts.ourRequests[peer.peerIdStr]; ok {
//		return fmt.Errorf("block already requested to this peer %s", peer.peerIdStr)
//	}
//	ts.ourRequests[peer.peerIdStr] = b
//	return nil
//}
//
//// RemovePeerRequest if request by the peer has been fulfilled
//func (ts *TorrentSession) RemovePeerRequest(peer *PeerConnection, b *BlockRequest) error {
//	ts.mu.Lock()
//	defer ts.mu.Unlock()
//
//	if _, ok := ts.peerRequests[peer.peerIdStr]; !ok {
//		return fmt.Errorf("this request by the peer %s doesn't exist", peer.peerIdStr)
//	}
//	delete(ts.peerRequests, peer.peerIdStr)
//	return nil
//}
//
//func (ts *TorrentSession) RemoveOurRequest(peer *PeerConnection, b *BlockRequest) error {
//	ts.mu.Lock()
//	defer ts.mu.Unlock()
//
//	if _, ok := ts.ourRequests[peer.peerIdStr]; !ok {
//		return fmt.Errorf("this request to the peer %s doesn't exist", peer.peerIdStr)
//	}
//	delete(ts.ourRequests, peer.peerIdStr)
//	return nil
//}

func (ts *TorrentSession) SendKeepAlive(peer *PeerConnection) (n int, err error) {
	message := NewKeepAliveMessage()
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'keep-alive' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendChoke(peer *PeerConnection) (n int, err error) {
	message := NewChokeMessage()
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'choke' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendUnchoke(peer *PeerConnection) (n int, err error) {
	message := NewUnchokeMessage()
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'unchoke' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendInterested(peer *PeerConnection) (n int, err error) {
	message := NewInterestedMessage()
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'interested' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendNotInterested(peer *PeerConnection) (n int, err error) {
	message := NewNotInterestedMessage()
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'not-interested' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendHave(pieceIndex uint32, peer *PeerConnection) (n int, err error) {
	message := NewHaveMessage(pieceIndex)
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'have' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendBitfield(peer *PeerConnection) (n int, err error) {
	message := NewBitfieldMessage(ts.bitfield)
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'bitfield' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendRequest(index uint32, begin uint32, length uint32, peer *PeerConnection) (n int, err error) {
	message := NewRequestMessage(index, begin, length)
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'request' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendPiece(index uint32, begin uint32, block []byte, peer *PeerConnection) (n int, err error) {
	message := NewPieceMessage(index, begin, block)
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'piece' message to peer %s: ", peer.peerIdStr)
	}
	return
}

func (ts *TorrentSession) SendCancel(index uint32, begin uint32, length uint32, peer *PeerConnection) (n int, err error) {
	message := NewCancelMessage(index, begin, length)
	n, err = peer.WriteMessage(message, ts)
	if err != nil {
		return 0, fmt.Errorf("error sending 'cancel' message to peer %s: ", peer.peerIdStr)
	}
	return
}
