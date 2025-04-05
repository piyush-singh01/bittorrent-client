package main

import (
	"errors"
	"io"
	"log"
	"net"
	"time"
)

const ConnectionBufferSize = 32768

const Reading = "reading"
const Writing = "writing"

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
			if isError := pc.errorHandler(err, session, nil, Reading); isError {
				continue
			}
			pc.PeerReaderMessageHandler(peerMessage, session)
			log.Printf("peer message is received from a peer %s and is of type %d", pc.peerIdStr, peerMessage.MessageId)
		}
	}
}

func (pc *PeerConnection) PeerWriter(session *TorrentSession) {
	pc.peerWriterStarted = true
	for {
		select {
		case <-pc.quitWriterChannel:
			log.Printf("quit peer writer signal received for %s, quitting", pc.peerIdStr)
			pc.peerWriterStarted = false
			return
		case <-time.After(session.configurable.keepAliveInterval):
			log.Printf("peer reader for %s is idle for 10 seconds, sending keep alive", pc.peerIdStr)
			_, err := pc.SendKeepAlive(session)
			pc.errorHandler(err, session, nil, Writing)
		case msg := <-pc.writeChannel:
			_, err := pc.WriteMessage(msg, session.rateTracker)
			pc.errorHandler(err, session, msg, Writing)
		}
	}
}

func (pc *PeerConnection) errorHandler(err error, session *TorrentSession, message *PeerMessage, errDuring string) bool {
	if err != nil {
		var netErr net.Error
		if err == io.EOF {
			log.Printf("error %s: %v connection gracefully closed by the peer %s", errDuring, err, pc.peerIdStr)
			session.quitChannel <- pc
		} else if errors.As(err, &netErr) && netErr.Temporary() {
			log.Printf("error %s: %v temperory network error with peer: %s", errDuring, err, pc.peerIdStr)
			if errDuring == Writing {
				pc.writeChannel <- message
			}
			// sends it back to the channel for write
		} else {
			log.Printf("error %s: %v. need to close connection with the peer: %s", errDuring, err, pc.peerIdStr)
			session.quitChannel <- pc
		}
		return true
	}
	return false
}

func (pc *PeerConnection) PeerReaderMessageHandler(peerMessage *PeerMessage, session *TorrentSession) {
	switch peerMessage.MessageId {
	case KeepAlive:
		log.Printf("keep alive message received from %s", pc.peerIdStr)
	case Choke:
		log.Printf("choke message received from %s", pc.peerIdStr)
		// stop the request pipeline goroutine
		// remove from the list of unchoked peers
	case Unchoke:
		log.Printf("unchoke message received from %s", pc.peerIdStr)
		// add to the list of unchoked peers
		// create a request pipeline to this peer
		//
	case Interested:
		log.Printf("interested message received from %s", pc.peerIdStr)
	case NotInterested:
		log.Printf("not-interested message received from %s", pc.peerIdStr)
	case Have:
		log.Printf("'have' message received from %s", pc.peerIdStr)
		have := peerMessage.GetHaveMessagePayload()
		pc.handleHaveMessage(have, session)
	case Bitfield:
		log.Printf("bitfield message received from %s", pc.peerIdStr)
		bitfield := peerMessage.GetBitfieldMessagePayload(session)
		if bitfield != nil {
			pc.handleBitfieldMessage(bitfield, session)
		}
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
	ts.connectedPeers.ReadOnlyIterate(func(peerIdStr string, connection *PeerConnection) bool {
		_, err := connection.sendMessage(peerMessage, ts)
		if err != nil {
			log.Printf(err.Error())
			return true
		}
		log.Printf("message sent to %s", peerIdStr)
		return true
	})
}

/************************************************** HANDLER METHODS **************************************************/

func (pc *PeerConnection) handleHaveMessage(have uint, session *TorrentSession) {
	pc.piecesMutex.Lock()
	defer pc.piecesMutex.Unlock()

	if pc.piecesBitfield != nil {
		pc.piecesBitfield.SetBit(have)
		session.bitfieldManager.AddPieceToExistingPeer(pc.peerIdStr, int(have))
	}

	if session.bitfieldManager.IsAmInterested(pc.peerIdStr) {
		pc.amInterested = true
		pc.writeChannel <- NewInterestedMessage()
	}

}

func (pc *PeerConnection) handleBitfieldMessage(bitfield *Bitset, session *TorrentSession) {
	pc.piecesMutex.Lock()
	pc.stateMutex.Lock()

	defer pc.piecesMutex.Unlock()
	defer pc.stateMutex.Unlock()

	if pc.piecesBitfield == nil {
		session.bitfieldManager.AddBitfieldToPeer(pc.peerIdStr, bitfield)
	} else {
		session.bitfieldManager.UpdateBitfieldForPeer(pc.peerIdStr, bitfield)
	}

	pc.piecesBitfield = bitfield
	log.Printf("checking if we are interested in the peer %s", pc.peerIdStr)
	if session.bitfieldManager.IsAmInterested(pc.peerIdStr) {
		log.Printf("we are interested in the peer %s", pc.peerIdStr)
		pc.amInterested = true
		pc.writeChannel <- NewInterestedMessage()
	}
}
