package main

import (
	"encoding/binary"
	"log"
	"math/bits"
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

func ParseAndValidateBitset(data []byte, selfBitfieldSize uint) (*Bitset, error) {
	size := uint(len(data) * 8)
	sizeBitset := ceilDiv(size, 64)

	bitValues := make([]uint64, sizeBitset)
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
		bitValues[i] = binary.BigEndian.Uint64(chunk)
	}

	peerBitset := &Bitset{
		bits: bitValues,
		size: size,
	}
	if peerBitset.Validate(selfBitfieldSize) {
		return peerBitset, nil
	}
	return nil, ErrBitsetSizeInvalid(selfBitfieldSize, peerBitset.size)
}

// Validate the receiver here is the peer bitfield
func (b *Bitset) Validate(selfBitfieldSize uint) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == selfBitfieldSize {
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
		log.Fatalf("bitset: out of bounds error")
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
		count += uint(bits.OnesCount64(b.bits[i]))
	}
	return count
}

func (b *Bitset) AnySetBits() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for i := range b.bits {
		if bits.OnesCount64(b.bits[i]) >= 1 {
			return true
		}
	}
	return false
}

func (b *Bitset) Size() uint {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.size
}

func (b *Bitset) And(other *Bitset) *Bitset {
	b.mu.RLock()
	other.mu.RLock()
	defer b.mu.RUnlock()
	defer other.mu.RUnlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not compute AND")
		return nil
	}

	return computeAnd(b, other)
}

// computeAnd assumes bitsets are of equal sizes
func computeAnd(x *Bitset, y *Bitset) *Bitset {
	result := NewBitset(x.size)
	for i := 0; i < len(result.bits); i++ {
		result.bits[i] = x.bits[i] & y.bits[i]
	}
	return result
}

func (b *Bitset) Or(other *Bitset) *Bitset {
	b.mu.RLock()
	other.mu.RLock()
	defer b.mu.RUnlock()
	defer other.mu.RUnlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not compute OR")
		return nil
	}

	return computeOr(b, other)
}

// computeOr assumes bitsets are of equal sizes
func computeOr(x *Bitset, y *Bitset) *Bitset {
	result := NewBitset(x.size)
	for i := 0; i < len(result.bits); i++ {
		result.bits[i] = x.bits[i] | y.bits[i]
	}
	return result
}

func (b *Bitset) Xor(other *Bitset) *Bitset {
	b.mu.RLock()
	other.mu.RLock()
	defer b.mu.RUnlock()
	defer other.mu.RUnlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not XOR")
		return nil
	}

	return computeXor(b, other)
}

func computeXor(x *Bitset, y *Bitset) *Bitset {
	result := NewBitset(x.size)
	for i := 0; i < len(result.bits); i++ {
		result.bits[i] = x.bits[i] ^ y.bits[i]
	}
	return result
}

func (b *Bitset) Not() *Bitset {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return computeNot(b)
}

func computeNot(x *Bitset) *Bitset {
	result := NewBitset(x.size)
	for i := 0; i < len(x.bits); i++ {
		result.bits[i] = ^x.bits[i]
	}

	remainder := x.size % 64
	if remainder != 0 {
		mask := ^uint64(0) << (64 - remainder)
		result.bits[len(result.bits)-1] &= mask
	}
	return result
}

func (b *Bitset) AndNot(other *Bitset) *Bitset {
	b.mu.RLock()
	other.mu.RLock()
	defer b.mu.RUnlock()
	defer other.mu.RUnlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not AND NOT")
		return nil
	}
	return computeAnd(b, computeNot(other))
}

func (b *Bitset) OrNot(other *Bitset) *Bitset {
	b.mu.RLock()
	other.mu.RLock()
	defer b.mu.RUnlock()
	defer other.mu.RUnlock()

	if b.size != other.size {
		log.Fatalf("[fatal]: bitset sizes are not equal, can not OR NOT")
		return nil
	}

	return computeOr(b, computeNot(other))
}
