package main

import (
	"bittorrent-client/structs"
	"sync"
)

/*
- What is it supposed to do
- - data structure containing:
- - - self bitfield
- - - all others bitfields
- - - mutexes for concurrency

- - To query
- - - If it contains some pieces that we do not have right now
- - - Get the rarest piece in the swarm
- - Updates to the data structures
- - - If a peer leaves a swarm, remove its info.
- - - If we receive a bitfield (peer joins a swarm), add its info.
- - - If we receive a Have, updates its info.
*/

type BitfieldManager struct {
	peerMutex      *structs.MutexMap[string, *sync.RWMutex]
	selfBitfield   *Bitset
	peerBitfields  *structs.MutexMap[string, *Bitset]
	pieceFrequency *structs.MutexAllForOne[int]
}

func NewBitfieldManager(selfBitfield *Bitset) *BitfieldManager {
	return &BitfieldManager{
		peerMutex:      structs.NewMutexMap[string, *sync.RWMutex](),
		selfBitfield:   selfBitfield,
		peerBitfields:  structs.NewMutexMap[string, *Bitset](),
		pieceFrequency: structs.NewAllForOne[int](),
	}
}

func (bm *BitfieldManager) AddPeer(peerIdStr string, peerBitfield *Bitset) {
	bm.peerMutex.PutOnlyIfNotExists(peerIdStr, new(sync.RWMutex))

	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()

	bm.peerBitfields.Put(peerIdStr, peerBitfield)

	for i := range peerBitfield.bits {
		for j := uint(0); j < uint(64); j++ {
			adjustedBitIndex := 63 - j
			if ((peerBitfield.bits[i] >> adjustedBitIndex) & 1) == 1 {
				pieceIndex := i*64 + int(j)
				bm.pieceFrequency.Inc(pieceIndex)
			}
		}
	}
}

func (bm *BitfieldManager) RemovePeer(peerIdStr string) {
	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()

	peerBitfield := bm.peerBitfields.GetOrDefault(peerIdStr)
	for i := range peerBitfield.bits {
		for j := uint(0); j < uint(64); j++ {
			adjustedBitIndex := 63 - j
			if ((peerBitfield.bits[i] >> adjustedBitIndex) & 1) == 1 {
				pieceIndex := i*64 + int(j)
				bm.pieceFrequency.Dec(pieceIndex)
			}
		}
	}

	bm.peerBitfields.Delete(peerIdStr)
	bm.peerMutex.Delete(peerIdStr)
}

func (bm *BitfieldManager) AddPieceToExistingPeer(peerIdStr string, pieceIndex int) {
	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()

	peerBitfield := bm.peerBitfields.GetOrDefault(peerIdStr)
	peerBitfield.SetBit(uint(pieceIndex))
	bm.pieceFrequency.Inc(pieceIndex)
}

func (bm *BitfieldManager) IsAmInterested(peerIdStr string) bool {
	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.RLock()
	defer peerMu.RUnlock()

	if bm.peerBitfields.GetOrDefault(peerIdStr).AndNot(bm.selfBitfield).AnySetBits() {
		return true
	}
	return false
}

// GetRarestPieceIndex find the most rare piece in swarm
func (bm *BitfieldManager) GetRarestPieceIndex() int {
	return bm.pieceFrequency.GetMostRareKey()
}
