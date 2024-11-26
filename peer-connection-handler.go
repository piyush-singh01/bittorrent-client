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
			log.Printf("quit peerConnection reader signal received for %s, quitting", pc.peerIdStr)
			pc.peerReaderStarted = false
			return
		default:
			peerMessage, _, err := pc.ReadMessage(session)
			if err != nil {
				var netErr net.Error
				if err == io.EOF {
					log.Printf("connection gracefully closed by the peerConnection %s", pc.peerIdStr)
					session.quitChannel <- pc
					return
				} else if errors.As(err, &netErr) && netErr.Temporary() {
					log.Printf("temperory network error with peerConnection: %s", pc.peerIdStr)
					// sends it back to the channel
					continue
				} else {
					log.Printf("error reading file. need to close connection with the peerConnection")
					session.quitChannel <- pc
					return
				}
			}
			log.Printf("peerConnection message is received from a peerConnection %s and is of type %d", pc.peerIdStr, peerMessage.MessageId)
			session.peerMessageChannel <- NewPeerConnectionMessage(peerMessage, pc)
		}
	}
}

func (pc *PeerConnection) PeerWriter(session *TorrentSession) {
	pc.peerWriterStarted = true
	for {
		select {
		case <-pc.quitWriterChannel:
			log.Printf("quit peerConnection writer signal received, quitting")
			pc.peerWriterStarted = false
			return
		case msg := <-pc.writeChannel:
			_, err := pc.WriteMessage(msg, session)
			if err != nil {
				var netErr net.Error
				if err == io.EOF {
					log.Printf("connection gracefully closed by the peerConnection %s", pc.peerIdStr)
					session.quitChannel <- pc
					return
				} else if errors.As(err, &netErr) && netErr.Temporary() {
					log.Printf("temperory network error with peerConnection: %s", pc.peerIdStr)
					pc.writeChannel <- msg
					// sends it back to the channel
					continue
				} else {
					log.Printf("error writing to file. need to close connection with the peerConnection")
					session.quitChannel <- pc
					return
				}
			}
			// but why do you need to send keep-alive here, you have already a keep alive handler in the main function
			//case <-time.After(time.Second * 10):
			//	// send keep alive
			//	_, err := session.SendKeepAlive(pc)
			//	log.Printf("sending keep alive")
			//	if err != nil {
			//		// todo: add a common error handler for checking if the connection has been terminated
			//		log.Printf("error sending keep alive, terminating connection")
			//		session.quitChannel <- pc // quit for now
			//	}
		}
	}
}

func (pc *PeerConnection) peerErrorHandler(session *TorrentSession) {

}

// HandleMessage n workers?? or what?
func (ts *TorrentSession) HandleMessage(handlerId int) {
	// add a case when torrent signals to quit
	for {
		select {
		case peerConnectionMessage := <-ts.peerMessageChannel:
			log.Printf("peerConnection connection message by %s received by message handler", peerConnectionMessage.peerConnection.peerIdStr)
			// add cases
			peerMessage := peerConnectionMessage.peerMessage
			peerConnection := peerConnectionMessage.peerConnection
			switch peerConnectionMessage.peerMessage.MessageId {
			case KeepAlive:
				log.Printf("received keep alive from peerConnection %s", peerConnection.peerIdStr)
			case Choke:
				log.Printf("received choke from peerConnection %s", peerConnection.peerIdStr)
				// remove it from the list from which the piece can be requested
				peerConnection.peerChoking = true
			case Unchoke:
				log.Printf("received unchoke from peerConnection %s", peerConnection.peerIdStr)
				// add it from the list from which the piece can be requested
				peerConnection.peerChoking = false
			case Interested:
				log.Printf("received interested from peerConnection %s", peerConnection.peerIdStr)
				// add it to the list of peers which can be unchoked
				peerConnection.peerInterested = true
				if err := ts.AddPeerToInterestedList(peerConnection); err != nil {
					log.Printf("error adding peerConnection to interested list: %v", err)
				}
			case NotInterested:
				log.Printf("received not interested from peerConnection %s", peerConnection.peerIdStr)
				peerConnection.peerInterested = false
				if err := ts.RemovePeerFromInterestedList(peerConnection); err != nil {
					log.Printf("error removing peerConnection from interested list: %v", err)
				}
			case Bitfield:
				log.Printf("received bitfield from peerConnection %s", peerConnection.peerIdStr)
				peerConnection.piecesBitfield = ParseBitset(peerMessage.Payload)
			case Request:
				log.Printf("received block request from peerConnection %s", peerConnection.peerIdStr)
			case Piece:
				log.Printf("piece block received from peerConnection %s", peerConnection.peerIdStr)
			case Cancel:
				log.Printf("received cancel block request from peerConnection %s", peerConnection.peerIdStr)
			default:
				log.Printf("unknown message type received from peerConnection %s, with message ID %d", peerConnection.peerIdStr, peerMessage.MessageId)
			}
		case <-time.After(time.Second * 5):
			log.Printf("the message handler with id %d is idle since 5 seconds", handlerId)
		}
	}
}
