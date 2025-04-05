package main

import (
	"bittorrent-client/structs"
	"time"
)

type Configurable struct {
	/* TCP-conn conf */
	tcpDialTimeout time.Duration
	listenerPort   uint16

	/* Keep Alive conf*/
	keepAliveInterval time.Duration
}

// TODO: Concurrency Control here??

type TorrentSession struct {
	torrent         *Torrent      // the parsed torrent
	configurable    *Configurable // configurations
	bitfield        *Bitset       // local bitfield
	rateTracker     *RateTracker  // the rate tracker for torrent session
	listener        *Listener     // listener for torrent session
	localPeerId     [20]byte      // local peer id
	trackerClient   *TrackerClient
	bitfieldManager *BitfieldManager

	connectedPeers *structs.MutexMap[string, *PeerConnection] // dictionary of peer connections, look up using peer id
	unchokedPeers  *structs.MutexMap[string, *PeerConnection] // dictionary of peer connections, that we have unchoked curerently
	//sentRequests

	quitChannel chan *PeerConnection // A quitter to terminate peer connections

	state *TorrentState
}

func NewTorrentSession(torrent *Torrent, localPeerId [20]byte) (*TorrentSession, error) {
	selfBitfield := NewBitset(torrent.Info.NumPieces)
	bitfieldManager := NewBitfieldManager(selfBitfield)

	connectedPeers := structs.NewMutexMap[string, *PeerConnection]()
	unchokedPeers := structs.NewMutexMap[string, *PeerConnection]()

	configurable := &Configurable{
		tcpDialTimeout:    time.Second * 5,
		listenerPort:      8888,
		keepAliveInterval: time.Second * 120,
	}

	return &TorrentSession{
		torrent:         torrent,
		configurable:    configurable,
		localPeerId:     localPeerId,
		bitfield:        selfBitfield,
		bitfieldManager: bitfieldManager,
		connectedPeers:  connectedPeers,
		unchokedPeers:   unchokedPeers,
		quitChannel:     make(chan *PeerConnection, 10),
		listener:        nil,
	}, nil
}

/* HANDLE PEER CONNECTION */

func (ts *TorrentSession) InitializePeer(peerConnection *PeerConnection) {
	peerConnection.mutex.Lock()
	defer peerConnection.mutex.Unlock()

	ts.connectedPeers.Put(peerConnection.peerIdStr, peerConnection)
	ts.bitfieldManager.AddPeerWithoutBitfield(peerConnection.peerIdStr)
	peerConnection.isActive = true

	peerConnection.writeChannel <- NewBitfieldMessage(ts.bitfield)
}

func (ts *TorrentSession) RemovePeer(peerConnection *PeerConnection) {
	peerConnection.mutex.Lock()
	defer peerConnection.mutex.Unlock()

	ts.connectedPeers.Delete(peerConnection.peerIdStr)
	ts.bitfieldManager.RemovePeer(peerConnection.peerIdStr)
	peerConnection.isActive = false
}

/* QUITTER GOROUTINE */

// StartQuitter Meant to run as a goroutine
func (ts *TorrentSession) StartQuitter() {
	for {
		connection := <-ts.quitChannel
		// TODO: if the connection is already closed, do not close it again
		connection.mutex.Lock()
		if connection.isActive {
			ts.RemovePeer(connection)
			connection.CloseConnection()
		}
		connection.mutex.Unlock()
	}
}

/* CLEAN UP */

func (ts *TorrentSession) CleanUp() {
	// stop all tickers
	// stop all goroutines on main
	// close all connections
}
