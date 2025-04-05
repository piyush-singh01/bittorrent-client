package main

import (
	"bittorrent-client/structs"
	"log"
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

func (bm *BitfieldManager) AddPeerWithoutBitfield(peerIdStr string) {
	log.Printf("configuring new peer %s in bitfield manager", peerIdStr)

	bm.peerMutex.PutOnlyIfNotExists(peerIdStr, new(sync.RWMutex))

	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()
}

func (bm *BitfieldManager) AddBitfieldToPeer(peerIdStr string, peerBitfield *Bitset) {
	log.Printf("adding bitfield to peer %s", peerIdStr)

	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()

	bm.peerBitfields.Put(peerIdStr, peerBitfield)
	bm.addBitfieldToFrequencyMap(peerBitfield)
}

func (bm *BitfieldManager) UpdateBitfieldForPeer(peerIdStr string, newPeerBitfield *Bitset) {
	log.Printf("updating the complete bitfield for peer %s", peerIdStr)

	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()

	existingPeerBitfield := bm.peerBitfields.GetOrDefault(peerIdStr)
	bm.removeBitfieldFromFrequencyMap(existingPeerBitfield)

	bm.peerBitfields.Put(peerIdStr, newPeerBitfield)
	bm.addBitfieldToFrequencyMap(newPeerBitfield)

}

func (bm *BitfieldManager) RemovePeer(peerIdStr string) {
	log.Printf("removing peer %s from bitfield manager", peerIdStr)

	peerMu := bm.peerMutex.GetOrDefault(peerIdStr)
	peerMu.Lock()
	defer peerMu.Unlock()

	peerBitfield := bm.peerBitfields.GetOrDefault(peerIdStr)
	bm.removeBitfieldFromFrequencyMap(peerBitfield)

	bm.peerBitfields.Delete(peerIdStr)
	bm.peerMutex.Delete(peerIdStr)
}

func (bm *BitfieldManager) AddPieceToExistingPeer(peerIdStr string, pieceIndex int) {
	log.Printf("adding piece %d to existing peer %s", pieceIndex, peerIdStr)

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

func (bm *BitfieldManager) addBitfieldToFrequencyMap(peerBitfield *Bitset) {
	if peerBitfield != nil {
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
}

func (bm *BitfieldManager) removeBitfieldFromFrequencyMap(peerBitfield *Bitset) {
	if peerBitfield != nil {
		for i := range peerBitfield.bits {
			for j := uint(0); j < uint(64); j++ {
				adjustedBitIndex := 63 - j
				if ((peerBitfield.bits[i] >> adjustedBitIndex) & 1) == 1 {
					pieceIndex := i*64 + int(j)
					bm.pieceFrequency.Dec(pieceIndex)
				}
			}
		}
	}
}
