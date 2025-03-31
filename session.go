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
	keepAliveTickInterval time.Duration
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

	connectedPeers structs.MutexMap[string, *PeerConnection] // dictionary of peer connections, look up using peer id
	unchokedPeers  structs.MutexMap[string, *PeerConnection] // dictionary of peer connections, that we have unchoked curerently
	//sentRequests

	quitChannel chan *PeerConnection // A quitter to terminate peer connections

	state *TorrentState

	/* Tickers */
	keepAliveTicker *time.Ticker
}

func NewTorrentSession(torrent *Torrent, localPeerId [20]byte) (*TorrentSession, error) {
	selfBitfield := NewBitset(torrent.Info.NumPieces)
	bitfieldManager := NewBitfieldManager(selfBitfield)
	configurable := &Configurable{
		tcpDialTimeout:        time.Second * 5,
		listenerPort:          8888,
		keepAliveTickInterval: time.Second * 120,
	}
	return &TorrentSession{
		torrent:         torrent,
		configurable:    configurable,
		localPeerId:     localPeerId,
		bitfield:        selfBitfield,
		bitfieldManager: bitfieldManager,
		quitChannel:     make(chan *PeerConnection, 10),
		listener:        nil,
	}, nil
}

/* HANDLE PEER CONNECTION */

func (ts *TorrentSession) InitializePeer(peerConnection *PeerConnection) {
	ts.connectedPeers.Put(peerConnection.peerIdStr, peerConnection)
	ts.bitfieldManager.AddPeer(peerConnection.peerIdStr, peerConnection.piecesBitfield)
	peerConnection.writeChannel <- NewBitfieldMessage(ts.bitfield)
}

func (ts *TorrentSession) RemovePeer(peerConnection *PeerConnection) {
	ts.connectedPeers.Delete(peerConnection.peerIdStr)
}

/* QUITTER GOROUTINE */

// StartQuitter Meant to run as a goroutine
func (ts *TorrentSession) StartQuitter() {
	for {
		connection := <-ts.quitChannel
		connection.CloseConnection()
	}
}

/* CLEAN UP */

func (ts *TorrentSession) CleanUp() {
	// stop all tickers
	// stop all goroutines on main
}
