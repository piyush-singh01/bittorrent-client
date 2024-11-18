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
		for peerIdStr, connection := range ts.peerConnection {
			_, err := ts.SendKeepAlive(connection)
			if err != nil {
				log.Printf("failed to send keep alive to %s: %v", peerIdStr, err)
				ts.quitChannel <- connection
				continue
			}
			log.Printf("keep alive sent to %s", peerIdStr)
		}
	}
}
