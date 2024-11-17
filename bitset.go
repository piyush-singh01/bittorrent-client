package main

import "encoding/binary"

type Bitset struct {
	bits []uint64
	size uint
}

func NewBitset(n uint) *Bitset {
	return &Bitset{
		bits: make([]uint64, ceilDiv(n, 64)),
		size: n,
	}
}

func ParseBitset(data []byte) *Bitset {
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

	return &Bitset{
		bits: bits,
		size: size,
	}
}

// Validate the receiver here is the peer bitfield
func (b *Bitset) Validate(torrentSession *TorrentSession) bool {
	if b.size == torrentSession.bitfield.size {
		return true
	}
	return false
}

func (b *Bitset) String() string {
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
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	b.bits[byteIndex] |= 1 << adjustedBitIndex
}

func (b *Bitset) ResetBit(v uint) {
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	b.bits[byteIndex] &= ^(1 << adjustedBitIndex)
}

func (b *Bitset) GetBit(v uint) uint {
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	return uint((b.bits[byteIndex] >> adjustedBitIndex) & 1)
}

func (b *Bitset) ToggleBit(v uint) {
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64

	adjustedBitIndex := 63 - bitIndex
	b.bits[byteIndex] ^= 1 << adjustedBitIndex
}

func (b *Bitset) Clear() {
	for i := range b.bits {
		b.bits[i] = 0
	}
}

func (b *Bitset) SetAll() {
	for i := range b.bits {
		b.bits[i] = ^uint64(0)
	}
}

func (b *Bitset) CountSetBits() uint {
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
	return b.size
}
