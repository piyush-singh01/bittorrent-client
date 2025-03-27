package main

import (
	"log"
	"time"
)

type Configurable struct {
	/* TCP-conn conf */
	initialTcpConnectionTimeout time.Duration
	listenerPort                uint16

	/* Keep Alive conf*/
	keepAliveTickInterval time.Duration

	/* Rate Tracker conf*/
	rateTrackerTickerInterval time.Duration
}

type TorrentSession struct {
	torrent      *Torrent      // the parsed torrent
	configurable *Configurable // configurations
	bitfield     *Bitset       // local bitfield
	rateTracker  *RateTracker  // the rate tracker for torrent session
	listener     *Listener     // listener for torrent session
	localPeerId  [20]byte      // local peer id

	peerConnection map[string]*PeerConnection // dictionary of peer connections, look up using peer id
	quitChannel    chan *PeerConnection       // A quitter to terminate peer connections

	/* Tickers */
	keepAliveTicker   *time.Ticker
	rateTrackerTicker *time.Ticker
}

func NewTorrentSession(torrent *Torrent, localPeerId [20]byte) (*TorrentSession, error) {
	configurable := &Configurable{
		initialTcpConnectionTimeout: time.Second * 5,
		listenerPort:                8888,
		keepAliveTickInterval:       time.Second * 120,
		rateTrackerTickerInterval:   time.Second,
	}
	return &TorrentSession{
		torrent:        torrent,
		configurable:   configurable,
		localPeerId:    localPeerId,
		peerConnection: make(map[string]*PeerConnection),
		rateTracker:    NewRateTracker(),
		bitfield:       NewBitset(torrent.Info.NumPieces),
		quitChannel:    make(chan *PeerConnection, 10),
		listener:       nil,
	}, nil
}

/* RATE TRACKER TICKER */

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

/* QUITTER GOROUTINE */

// StartQuitter Meant to run as a goroutine
func (ts *TorrentSession) StartQuitter() {
	for {
		connection := <-ts.quitChannel
		connection.CloseConnection(ts)
	}
}

/* CLEAN UP */

func (ts *TorrentSession) CleanUp() {
	// stop all tickers
}
