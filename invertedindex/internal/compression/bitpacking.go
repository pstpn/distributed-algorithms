package compression

import (
	"encoding/binary"

	"github.com/dataence/encoding/bitpacking"
)

const bpBlock = 32
const rawFlag uint32 = 0x80000000

func maxBits(data []uint32) uint8 {
	m := uint32(0)
	for _, v := range data {
		if v > m {
			m = v
		}
	}
	if m == 0 {
		return 1
	}
	b := uint8(0)
	for m > 0 {
		b++
		m >>= 1
	}
	return b
}

func uvarintSize(v uint32) int {
	if v < 0x80 {
		return 1
	}
	if v < 0x4000 {
		return 2
	}
	if v < 0x200000 {
		return 3
	}
	if v < 0x10000000 {
		return 4
	}
	return 5
}

func packBlock(values []uint32) []byte {
	bits := maxBits(values)

	padded := make([]int32, bpBlock)
	for i, v := range values {
		padded[i] = int32(v)
	}

	out := make([]int32, int(bits))
	bitpacking.FastPack(padded, 0, out, 0, int(bits))

	buf := make([]byte, 1+int(bits)*4)
	buf[0] = byte(bits)
	le := binary.LittleEndian
	for i, v := range out {
		le.PutUint32(buf[1+i*4:], uint32(v))
	}
	return buf
}

func unpackBlockInto(data []byte, result []uint32, resultOff int) int {
	bits := uint8(data[0])
	in := make([]int32, int(bits))
	le := binary.LittleEndian
	for i := 0; i < int(bits); i++ {
		in[i] = int32(le.Uint32(data[1+i*4:]))
	}

	out := make([]int32, bpBlock)
	bitpacking.FastUnpack(in, 0, out, 0, int(bits))

	remaining := len(result) - resultOff
	if remaining > bpBlock {
		remaining = bpBlock
	}
	for i := 0; i < remaining; i++ {
		result[resultOff+i] = uint32(out[i])
	}

	return 1 + int(bits)*4
}

func bpCompressedSize(arr []uint32) int {
	size := 8
	for i := 0; i < len(arr); i += bpBlock {
		end := i + bpBlock
		if end > len(arr) {
			end = len(arr)
		}
		bits := maxBits(arr[i:end])
		size += 1 + int(bits)*4
	}
	return size
}

func varintCompressedSize(arr []uint32) int {
	size := 8
	for _, v := range arr {
		size += uvarintSize(v)
	}
	return size
}

func Compress(arr []uint32) []byte {
	if len(arr) == 0 {
		buf := make([]byte, 8)
		le := binary.LittleEndian
		le.PutUint32(buf[0:4], 8)
		return buf
	}

	le := binary.LittleEndian

	bpSize := bpCompressedSize(arr)
	viSize := varintCompressedSize(arr)

	if viSize <= bpSize {
		out := make([]byte, viSize)
		le.PutUint32(out[0:4], uint32(viSize))
		le.PutUint32(out[4:8], uint32(len(arr))|rawFlag)
		off := 8
		for _, v := range arr {
			off += binary.PutUvarint(out[off:], uint64(v))
		}
		return out
	}

	var blocks []byte
	for i := 0; i < len(arr); i += bpBlock {
		end := i + bpBlock
		if end > len(arr) {
			end = len(arr)
		}
		blocks = append(blocks, packBlock(arr[i:end])...)
	}

	out := make([]byte, 4+4+len(blocks))
	le.PutUint32(out[0:4], uint32(len(out)))
	le.PutUint32(out[4:8], uint32(len(arr)))
	copy(out[8:], blocks)

	return out
}

func Decompress(data []byte) []uint32 {
	if len(data) == 0 {
		return nil
	}
	le := binary.LittleEndian
	totalSize := int(le.Uint32(data[0:4]))
	if totalSize < 8 {
		return nil
	}
	countRaw := le.Uint32(data[4:8])
	n := int(countRaw &^ rawFlag)

	if n == 0 {
		return nil
	}

	if countRaw&rawFlag != 0 {
		result := make([]uint32, n)
		off := 8
		for i := 0; i < n; i++ {
			v, nb := binary.Uvarint(data[off:])
			result[i] = uint32(v)
			off += nb
		}
		return result
	}

	result := make([]uint32, n)
	off := 8
	resultOff := 0

	for resultOff < n {
		consumed := unpackBlockInto(data[off:], result, resultOff)
		off += consumed
		resultOff += bpBlock
	}

	return result
}

func CompressedSize(data []byte) int {
	if len(data) < 4 {
		return 0
	}
	le := binary.LittleEndian
	return int(le.Uint32(data[0:4]))
}
