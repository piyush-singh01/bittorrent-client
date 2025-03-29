package main

import (
	"errors"
	"io"
	"log"
	"net"
	"time"
)

const ConnectionBufferSize = 32768

func (pc *PeerConnection) PeerReader(session *TorrentSession) {
	pc.peerReaderStarted = true
	for {
		select {
		case <-pc.quitReaderChannel:
			log.Printf("quit peer reader signal received for %s, quitting", pc.peerIdStr)
			pc.peerReaderStarted = false
			return
		case <-time.After(10 * time.Second):
			log.Printf("peer reader for %s is idle for 10 secons", pc.peerIdStr)
		default:
			peerMessage, _, err := pc.ReadMessage(session.rateTracker)
			if err != nil {
				var netErr net.Error
				if err == io.EOF {
					log.Printf("connection gracefully closed by the peer %s", pc.peerIdStr)
					session.quitChannel <- pc
					return
				} else if errors.As(err, &netErr) && netErr.Temporary() {
					log.Printf("temperory network error with peer: %s while reading message", pc.peerIdStr)
					continue
				} else {
					log.Printf("error reading file. need to close connection with the peer")
					session.quitChannel <- pc
					return
				}
			}
			pc.PeerReaderMessageHandler(peerMessage)
			log.Printf("peer message is received from a peer %s and is of type %d", pc.peerIdStr, peerMessage.MessageId)
		}
	}
}

func (pc *PeerConnection) PeerWriter(session *TorrentSession) {
	pc.peerWriterStarted = true
	for {
		select {
		case <-pc.quitWriterChannel:
			log.Printf("quit peer writer signal received, quitting")
			pc.peerWriterStarted = false
			return
		case msg := <-pc.writeChannel:
			_, err := pc.WriteMessage(msg, session.rateTracker)
			if err != nil {
				var netErr net.Error
				if err == io.EOF {
					log.Printf("connection gracefully closed by the peer %s", pc.peerIdStr)
					session.quitChannel <- pc
					return
				} else if errors.As(err, &netErr) && netErr.Temporary() {
					log.Printf("temperory network error with peer: %s while writing message", pc.peerIdStr)
					pc.writeChannel <- msg
					// sends it back to the channel
					continue
				} else {
					log.Printf("error writing to file. need to close connection with the peer")
					session.quitChannel <- pc
					return
				}
			}
		}
	}
}

func (pc *PeerConnection) PeerReaderMessageHandler(peerMessage *PeerMessage) {
	switch peerMessage.MessageId {
	case KeepAlive:
		log.Printf("keep alive message received from %s", pc.peerIdStr)
	case Choke:
		log.Printf("choke message received from %s", pc.peerIdStr)
	case Unchoke:
		log.Printf("unchoke message received from %s", pc.peerIdStr)
	case Interested:
		log.Printf("interested message received from %s", pc.peerIdStr)
	case NotInterested:
		log.Printf("not-interested message received from %s", pc.peerIdStr)
	case Have:
		log.Printf("'have' message received from %s", pc.peerIdStr)
		if pc.piecesBitfield != nil {
			pc.piecesBitfield.SetBit(peerMessage.GetHaveMessagePayload())
		}
	case Bitfield:
		log.Printf("bitfield message received from %s", pc.peerIdStr)
		if pc.piecesBitfield == nil {
			pc.piecesBitfield = peerMessage.GetBitfieldMessagePayload()
		}
		// if we are interested in this peer:
		//		send interested to write channel of this connection
	case Request:
		log.Printf("request message received from %s", pc.peerIdStr)
	case Piece:
		log.Printf("piece message received from %s", pc.peerIdStr)
	case Cancel:
		log.Printf("cancel message received from %s", pc.peerIdStr)
	default:
		log.Printf("unknown message received from %s", pc.peerIdStr)
	}
}

func (ts *TorrentSession) BroadcastMessage(peerMessage *PeerMessage) {
	log.Printf("broadcasting message %d", peerMessage.MessageId)
	ts.connectedPeers.Range(func(key, value interface{}) bool {
		peerIdStr := key.(string)
		connection := value.(*PeerConnection)
		if time.Since(connection.lastWriteTime) >= ts.configurable.keepAliveTickInterval {
			_, err := connection.sendMessage(peerMessage, ts)
			if err != nil {
				log.Printf(err.Error())
				return true
			}
			log.Printf("message sent to %s", peerIdStr)
		}
		return true
	})
}
