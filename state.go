package main

import "sync"

type StateRequestType int

const (
	Uploaded StateRequestType = iota
	Downloaded
	Left
)

type TorrentState struct {
	mu         sync.RWMutex
	left       int64
	downloaded int64
	uploaded   int64

	stateChannel chan Pair[StateRequestType, int64]
}

func NewTorrentState(totalLength int64) *TorrentState {
	return &TorrentState{
		stateChannel: make(chan Pair[StateRequestType, int64], 10),
		left:         totalLength,
		downloaded:   0,
		uploaded:     0,
	}
}

// StateHandler Meant to be run as a goroutine
func (st *TorrentState) StateHandler() {
	for {
		pieceInfo := <-st.stateChannel
		stateRequestType := pieceInfo.first
		length := pieceInfo.second

		st.mu.Lock()
		if stateRequestType == Uploaded {
			st.uploaded += length
		} else if stateRequestType == Downloaded {
			st.downloaded += length
		} else if stateRequestType == Left {
			st.left -= length
		}
		st.mu.Unlock()
	}
}

func (st *TorrentState) GetState() (int64, int64, int64) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.left, st.downloaded, st.uploaded
}
