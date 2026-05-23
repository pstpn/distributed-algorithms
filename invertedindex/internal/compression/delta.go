package compression

import "encoding/binary"

type PostingEntry struct {
	DocID     uint32
	Positions []uint32
}

func DeltaEncodePostings(entries []PostingEntry) []byte {
	if len(entries) == 0 {
		return nil
	}

	estimatedSize := 5
	for _, entry := range entries {
		estimatedSize += 5 * 2
		estimatedSize += len(entry.Positions) * 5
	}
	buf := make([]byte, 0, estimatedSize)

	var tmp [binary.MaxVarintLen64]byte

	n := binary.PutUvarint(tmp[:], uint64(len(entries)))
	buf = append(buf, tmp[:n]...)

	prevDocID := uint64(0)
	for _, entry := range entries {
		docIDDelta := uint64(entry.DocID) - prevDocID
		prevDocID = uint64(entry.DocID)

		n = binary.PutUvarint(tmp[:], docIDDelta)
		buf = append(buf, tmp[:n]...)

		n = binary.PutUvarint(tmp[:], uint64(len(entry.Positions)))
		buf = append(buf, tmp[:n]...)

		if len(entry.Positions) > 0 {
			prevPos := uint64(0)
			for _, pos := range entry.Positions {
				posDelta := uint64(pos) - prevPos
				prevPos = uint64(pos)
				n = binary.PutUvarint(tmp[:], posDelta)
				buf = append(buf, tmp[:n]...)
			}
		}
	}

	return buf
}

func DeltaDecodePostings(data []byte) []PostingEntry {
	if len(data) == 0 {
		return nil
	}

	offset := 0

	numDocs, n := binary.Uvarint(data[offset:])
	offset += n

	result := make([]PostingEntry, 0, numDocs)
	currentDocID := uint64(0)

	for i := 0; i < int(numDocs); i++ {
		docIDDelta, n := binary.Uvarint(data[offset:])
		offset += n
		currentDocID += docIDDelta

		tf, n := binary.Uvarint(data[offset:])
		offset += n

		positions := make([]uint32, 0, tf)
		currentPos := uint64(0)
		for j := 0; j < int(tf); j++ {
			posDelta, n := binary.Uvarint(data[offset:])
			offset += n
			currentPos += posDelta
			positions = append(positions, uint32(currentPos))
		}

		result = append(result, PostingEntry{
			DocID:     uint32(currentDocID),
			Positions: positions,
		})
	}

	return result
}
