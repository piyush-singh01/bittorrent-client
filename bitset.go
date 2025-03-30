package main

import (
	"encoding/binary"
	"log"
	"sync"
)

type Bitset struct {
	mu   sync.RWMutex
	bits []uint64
	size uint
}

func NewBitset(n uint) *Bitset {
	return &Bitset{
		bits: make([]uint64, ceilDiv(n, 64)),
		size: n,
	}
}

func ParseBitset(data []byte, selfBitfield *Bitset) (*Bitset, error) {
	size := uint(len(data) * 8)
	sizeBitset := ceilDiv(size, 64)

	bits := make([]uint64, sizeBitset)
	for i := uint(0); i < sizeBitset; i++ {
		start := i * 8
		end := (i + 1) * 8
		if end > uint(len(data)) {
			end = uint(len(data))
		}

		chunk := data[start:end]
		if len(chunk) < 8 {
			paddedChunk := make([]byte, 8)
			copy(paddedChunk, chunk)
			chunk = paddedChunk
		}
		bits[i] = binary.BigEndian.Uint64(chunk)
	}

	peerBitset := &Bitset{
		bits: bits,
		size: size,
	}
	if peerBitset.Validate(selfBitfield) {
		return peerBitset, nil
	}
	return nil, ErrBitsetSizeInvalid(selfBitfield.size, peerBitset.size)
}

// Validate the receiver here is the peer bitfield
func (b *Bitset) Validate(selfBitfield *Bitset) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == selfBitfield.size {
		return true
	}
	return false
}

func (b *Bitset) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var res = ""
	for v := uint(0); v < b.size; v++ {
		byteIndex := v / 64
		bitIndex := v % 64

		adjustedBitIndex := 63 - bitIndex
		if ((b.bits[byteIndex] >> adjustedBitIndex) & 1) == 1 {
			res += "1"
		} else {
			res += "0"
		}
	}
	return res
}

func (b *Bitset) Serialize() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var res = make([]byte, len(b.bits)*8)
	for i, ele := range b.bits {
		binary.BigEndian.PutUint64(res[i*8:(i+1)*8], ele)
	}
	return res
}

func (b *Bitset) checkOutOfBounds(v uint) {
	if v >= b.size {
		panic("out of bounds error")
	}
}

func (b *Bitset) SetBit(v uint) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	b.bits[byteIndex] |= 1 << adjustedBitIndex
}

func (b *Bitset) ResetBit(v uint) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	b.bits[byteIndex] &= ^(1 << adjustedBitIndex)
}

func (b *Bitset) GetBit(v uint) uint {
	b.mu.RLock()
	defer b.mu.RUnlock()

	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	return uint((b.bits[byteIndex] >> adjustedBitIndex) & 1)
}

func (b *Bitset) ToggleBit(v uint) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	b.bits[byteIndex] ^= 1 << adjustedBitIndex
}

func (b *Bitset) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.bits {
		b.bits[i] = 0
	}
}

func (b *Bitset) SetAll() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.bits {
		b.bits[i] = ^uint64(0)
	}
}

func (b *Bitset) CountSetBits() uint {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var count uint = 0
	for i := range b.bits {
		for j := uint(0); j < uint(64); j++ {
			if ((b.bits[i] >> j) & 1) == 1 {
				count++
			}
		}
	}
	return count
}

func (b *Bitset) Size() uint {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.size
}

func (b *Bitset) And(other *Bitset) *Bitset {
	b.mu.Lock()
	other.mu.Lock()
	defer b.mu.Unlock()
	defer other.mu.Unlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not compute AND")
		return nil
	}

	result := NewBitset(b.size)
	for i := 0; i < len(result.bits); i++ {
		result.bits[i] = b.bits[i] & other.bits[i]
	}
	return result
}

func (b *Bitset) Or(other *Bitset) *Bitset {
	b.mu.Lock()
	other.mu.Lock()
	defer b.mu.Unlock()
	defer other.mu.Unlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not compute OR")
		return nil
	}

	result := NewBitset(b.size)
	for i := 0; i < len(result.bits); i++ {
		result.bits[i] = b.bits[i] | other.bits[i]
	}
	return result
}

func (b *Bitset) Xor(other *Bitset) *Bitset {
	b.mu.Lock()
	other.mu.Lock()
	defer b.mu.Unlock()
	defer other.mu.Unlock()
	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not XOR")
		return nil
	}

	result := NewBitset(b.size)
	for i := 0; i < len(result.bits); i++ {
		result.bits[i] = b.bits[i] ^ other.bits[i]
	}
	return result
}

func (b *Bitset) Not() *Bitset {
	result := NewBitset(b.size)

	b.mu.Lock()
	defer b.mu.Unlock()

	for i := 0; i < len(b.bits); i++ {
		result.bits[i] = ^b.bits[i]
	}

	remainder := b.size % 64
	if remainder != 0 {
		mask := ^uint64(0) << (64 - remainder)
		result.bits[len(result.bits)-1] &= mask
	}
	return result

}

func (b *Bitset) AndNot(other *Bitset) *Bitset {
	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not AND NOT")
		return nil
	}
	return b.And(other.Not())
}

func (b *Bitset) OrNot(other *Bitset) *Bitset {
	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not OR NOT")
		return nil
	}
	return b.Or(other.Not())
}
