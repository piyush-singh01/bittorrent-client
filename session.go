package main

import (
	"sync"
	"time"
)

type Configurable struct {
	/* TCP-conn conf */
	tcpDialTimeout time.Duration
	listenerPort   uint16

	/* Keep Alive conf*/
	keepAliveTickInterval time.Duration
}

type TorrentSession struct {
	torrent       *Torrent      // the parsed torrent
	configurable  *Configurable // configurations
	bitfield      *Bitset       // local bitfield
	rateTracker   *RateTracker  // the rate tracker for torrent session
	listener      *Listener     // listener for torrent session
	localPeerId   [20]byte      // local peer id
	trackerClient *TrackerClient

	connectedPeers sync.Map // dictionary of peer connections, look up using peer id
	unchokedPeers  sync.Map
	//sentRequests

	quitChannel chan *PeerConnection // A quitter to terminate peer connections

	state *TorrentState

	/* Tickers */
	keepAliveTicker *time.Ticker
}

func NewTorrentSession(torrent *Torrent, localPeerId [20]byte) (*TorrentSession, error) {
	configurable := &Configurable{
		tcpDialTimeout:        time.Second * 5,
		listenerPort:          8888,
		keepAliveTickInterval: time.Second * 120,
	}
	return &TorrentSession{
		torrent:      torrent,
		configurable: configurable,
		localPeerId:  localPeerId,
		bitfield:     NewBitset(torrent.Info.NumPieces),
		quitChannel:  make(chan *PeerConnection, 10),
		listener:     nil,
	}, nil
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
