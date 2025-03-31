package main

import (
	"log"
	"time"
)

func (ts *TorrentSession) StartKeepAliveTicker() {
	if ts.keepAliveTicker != nil {
		ts.keepAliveTicker.Stop()
	}
	ts.keepAliveTicker = time.NewTicker(ts.configurable.keepAliveTickInterval)
}

func (ts *TorrentSession) KeepAliveHandler() {
	for {
		<-ts.keepAliveTicker.C
		ts.connectedPeers.Iterate(func(peerIdStr string, connection *PeerConnection) bool {
			if time.Since(connection.lastWriteTime) >= ts.configurable.keepAliveTickInterval {
				_, err := connection.SendKeepAlive(ts)
				if err != nil {
					log.Printf("failed to send keep alive to %s: %v: sending connection to quit channel", peerIdStr, err)
					ts.quitChannel <- connection
					return true
				}
				log.Printf("keep alive sent to %s", peerIdStr)
			}
			return true
		})
	}
}
