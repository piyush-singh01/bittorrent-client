package main

import (
	"fmt"
	"log"
)

// TODO: We are passing the session only for rate tracker,
// pass in the rate tracker only later on
// instead of the whole session pointer

func (pc *PeerConnection) SendKeepAlive(session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewKeepAliveMessage(), session)
	if err != nil {
		log.Printf("error sending 'keep-alive' to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'keep-alive' to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendChoke(session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewChokeMessage(), session)
	if err != nil {
		log.Printf("error sending 'choke' to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'choke' to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendUnchoke(session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewUnchokeMessage(), session)
	if err != nil {
		log.Printf("error sending 'unchoke' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'unchoke' to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendInterested(session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewInterestedMessage(), session)
	if err != nil {
		log.Printf("error sending 'interested' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'interested' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendNotInterested(session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewNotInterestedMessage(), session)
	if err != nil {
		log.Printf("error sending 'not-interested' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'not-interested' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendHave(pieceIndex uint32, session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewHaveMessage(pieceIndex), session)
	if err != nil {
		log.Printf("error sending 'have' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'have' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendBitfield(session *TorrentSession) (n int, err error) {
	if session.bitfield == nil {
		return 0, fmt.Errorf("local bitfield is nil, can not send bitfield")
	}

	n, err = pc.WriteMessage(NewBitfieldMessage(session.bitfield), session)
	if err != nil {
		log.Printf("error sending 'bitfield' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'bitfield' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendRequest(index uint32, begin uint32, length uint32, session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewRequestMessage(index, begin, length), session)
	if err != nil {
		log.Printf("error sending 'request' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'request' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendPiece(index uint32, begin uint32, block []byte, session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewPieceMessage(index, begin, block), session)
	if err != nil {
		log.Printf("error sending 'piece' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'piece' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}

func (pc *PeerConnection) SendCancel(index uint32, begin uint32, length uint32, session *TorrentSession) (n int, err error) {
	n, err = pc.WriteMessage(NewCancelMessage(index, begin, length), session)
	if err != nil {
		log.Printf("error sending 'cancel' message to peer %s: %v", pc.peerIdStr, err)
		return 0, fmt.Errorf("error sending 'cancel' message to peer %s: %v", pc.peerIdStr, err)
	}
	return
}
