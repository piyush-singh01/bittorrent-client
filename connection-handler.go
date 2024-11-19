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
			return
		default:
			var buf = make([]byte, ConnectionBufferSize)
			n, err := pc.tcpConn.Read(buf)

			if err != nil {
				var netErr net.Error
				if err == io.EOF {
					log.Printf("connection gracefully closed by the peer %s", pc.peerIdStr)
					session.quitChannel <- pc
					return
				} else if errors.As(err, &netErr) && netErr.Temporary() {
					log.Printf("temperory network error with peer: %s", pc.peerIdStr)
					// sends it back to the channel
					continue
				} else {
					log.Printf("error reading file. need to close connection with the peer")
					session.quitChannel <- pc
					return
				}
			}
			peerMessage := ParsePeerMessage(buf[:n])
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
			return
		case msg := <-pc.writeChannel:
			n, err := pc.tcpConn.Write(msg.Serialize())
			if err != nil {
				var netErr net.Error
				if err == io.EOF {
					log.Printf("connection gracefully closed by the peer %s", pc.peerIdStr)
					session.quitChannel <- pc
					return
				} else if errors.As(err, &netErr) && netErr.Temporary() {
					log.Printf("temperory network error with peer: %s", pc.peerIdStr)
					// sends it back to the channel
					continue
				} else {
					log.Printf("error writing to file. need to close connection with the peer")
					session.quitChannel <- pc
					return
				}
			}
			pc.UpdateLastWriteTime()
			log.Printf("%d bytes written to the peer %s", n, pc.peerIdStr)
		case <-time.After(time.Second * 5):
			// send keep alive
			_, err := session.SendKeepAlive(pc)
			if err != nil {
				// todo: add a common error handler for checking if the connection has been terminated
				log.Printf("error sending keep alive, terminating connection")
				session.quitChannel <- pc // quit for now
			}
		}
	}
}
