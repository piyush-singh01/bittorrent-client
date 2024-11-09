package main

type Bitset struct {
	bits []uint64
	size uint
}

func NewBitset(n uint) *Bitset {
	return &Bitset{
		bits: make([]uint64, ceilDiv(n, 64)), // wrong ceil_div(n, 64)
		size: n,
	}
}

func (b *Bitset) String() string {
	res := ""
	var i uint
	for i = 0; i < b.size; i++ {
		if b.GetBit(i) == 1 {
			res += "1"
		} else {
			res += "0"
		}
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
	b.bits[byteIndex] |= 1 << bitIndex
}

func (b *Bitset) ResetBit(v uint) {
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64
	b.bits[byteIndex] &= ^(1 << bitIndex)
}

func (b *Bitset) GetBit(v uint) uint {
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64
	return uint((b.bits[byteIndex] >> bitIndex) & 1)
}

func (b *Bitset) ToggleBit(v uint) {
	b.checkOutOfBounds(v)

	byteIndex := v / 64
	bitIndex := v % 64
	b.bits[byteIndex] ^= 1 << bitIndex
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
