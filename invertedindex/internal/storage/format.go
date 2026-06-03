package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/pstpn/iidx/internal/compression"
)

const (
	magic      = "INVX"
	headerSize = 40
)

type Header struct {
	Magic          [4]byte
	NumTerms       uint32
	NumDocs        uint32
	TotalTokens    uint64
	PostingsOffset int64
	DocLengthsSize uint32
}

func writeHeader(w io.Writer, h *Header) error {
	buf := make([]byte, headerSize)
	copy(buf[0:4], h.Magic[:])
	le := binary.LittleEndian
	le.PutUint32(buf[4:8], h.NumTerms)
	le.PutUint32(buf[8:12], h.NumDocs)
	le.PutUint64(buf[12:20], h.TotalTokens)
	le.PutUint64(buf[20:28], uint64(h.PostingsOffset))
	le.PutUint32(buf[28:32], h.DocLengthsSize)
	_, err := w.Write(buf)
	return err
}

func ReadHeader(s *MMapStorage) (*Header, error) {
	buf := s.Slice(0, headerSize)
	h := &Header{}
	copy(h.Magic[:], buf[0:4])
	le := binary.LittleEndian
	h.NumTerms = le.Uint32(buf[4:8])
	h.NumDocs = le.Uint32(buf[8:12])
	h.TotalTokens = le.Uint64(buf[12:20])
	h.PostingsOffset = int64(le.Uint64(buf[20:28]))
	h.DocLengthsSize = le.Uint32(buf[28:32])
	if string(h.Magic[:]) != magic {
		return nil, fmt.Errorf("invalid magic: expected %q, got %q", magic, h.Magic[:])
	}
	return h, nil
}

type TermEntry struct {
	Term           string
	DocFreq        uint32
	PostingsOffset int64
	PostingsLength int32
}

type DocEntry struct {
	Title string
	Text  string
}

type IndexWriter struct {
	file        *os.File
	header      Header
	termEntries []TermEntry
	postings    [][]byte
	docLengths  []uint32
}

func NewIndexWriter(filename string, numDocs uint32, totalTokens uint64, docLengths []uint32) (*IndexWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create index file %s: %w", filename, err)
	}
	return &IndexWriter{
		file:       file,
		docLengths: docLengths,
		header: Header{
			NumDocs:     numDocs,
			TotalTokens: totalTokens,
		},
	}, nil
}

func (w *IndexWriter) AddTerm(term string, docFreq uint32, compressedPostings []byte) {
	w.termEntries = append(w.termEntries, TermEntry{
		Term:    term,
		DocFreq: docFreq,
	})
	w.postings = append(w.postings, compressedPostings)
	w.header.NumTerms++
}

func (w *IndexWriter) Write() error {
	defer w.file.Close()
	le := binary.LittleEndian

	copy(w.header.Magic[:], magic)

	docFreqs := make([]uint32, len(w.termEntries))
	postingsLengths := make([]uint32, len(w.termEntries))
	relOffsets := make([]uint32, len(w.termEntries))

	cumOffset := uint32(0)
	for i := range w.termEntries {
		docFreqs[i] = w.termEntries[i].DocFreq
		postingsLengths[i] = uint32(len(w.postings[i]))
		relOffsets[i] = cumOffset
		cumOffset += uint32(len(w.postings[i]))
	}

	offsetDeltas := make([]uint32, len(relOffsets))
	prev := uint32(0)
	for i, off := range relOffsets {
		offsetDeltas[i] = off - prev
		prev = off
	}

	compressedDocFreqs := compression.Compress(docFreqs)
	compressedOffsetDeltas := compression.Compress(offsetDeltas)
	compressedPostingsLengths := compression.Compress(postingsLengths)
	compressedDocLengths := compression.Compress(w.docLengths)

	w.header.DocLengthsSize = uint32(len(compressedDocLengths))

	termStringsSize := int64(0)
	for _, entry := range w.termEntries {
		termStringsSize += int64(2 + len(entry.Term))
	}

	compressedArraysSize := int64(len(compressedDocFreqs) + len(compressedOffsetDeltas) + len(compressedPostingsLengths))

	w.header.PostingsOffset = headerSize + termStringsSize + compressedArraysSize + int64(len(compressedDocLengths))

	if _, err := w.file.Seek(headerSize, io.SeekStart); err != nil {
		return fmt.Errorf("seek past header: %w", err)
	}

	bw := bufio.NewWriterSize(w.file, 256*1024)

	for _, entry := range w.termEntries {
		var lenBuf [2]byte
		le.PutUint16(lenBuf[:], uint16(len(entry.Term)))
		if _, err := bw.Write(lenBuf[:]); err != nil {
			return fmt.Errorf("write term length: %w", err)
		}
		if _, err := bw.WriteString(entry.Term); err != nil {
			return fmt.Errorf("write term: %w", err)
		}
	}

	if _, err := bw.Write(compressedDocFreqs); err != nil {
		return fmt.Errorf("write doc freqs: %w", err)
	}
	if _, err := bw.Write(compressedOffsetDeltas); err != nil {
		return fmt.Errorf("write offset deltas: %w", err)
	}
	if _, err := bw.Write(compressedPostingsLengths); err != nil {
		return fmt.Errorf("write postings lengths: %w", err)
	}
	if _, err := bw.Write(compressedDocLengths); err != nil {
		return fmt.Errorf("write doc lengths: %w", err)
	}

	for i, postingData := range w.postings {
		if _, err := bw.Write(postingData); err != nil {
			return fmt.Errorf("write postings for term %q: %w", w.termEntries[i].Term, err)
		}
	}

	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush buffer: %w", err)
	}

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek to header: %w", err)
	}
	if err := writeHeader(w.file, &w.header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	return w.file.Sync()
}

func ReadTermEntries(s *MMapStorage, header *Header) ([]TermEntry, error) {
	offset := int64(headerSize)
	le := binary.LittleEndian

	terms := make([]string, 0, header.NumTerms)
	for i := uint32(0); i < header.NumTerms; i++ {
		lenBuf := s.Slice(offset, 2)
		termLen := le.Uint16(lenBuf)
		offset += 2

		termBuf := s.Slice(offset, int(termLen))
		offset += int64(termLen)
		terms = append(terms, string(termBuf))
	}

	compressedSectionEnd := header.PostingsOffset - int64(header.DocLengthsSize)
	compressedSectionSize := compressedSectionEnd - offset
	if compressedSectionSize <= 0 {
		return nil, fmt.Errorf("invalid compressed section size: %d", compressedSectionSize)
	}

	buf := s.Slice(offset, int(compressedSectionSize))

	off := 0
	docFreqs := compression.Decompress(buf[off:])
	off += compression.CompressedSize(buf[off:])

	offsetDeltas := compression.Decompress(buf[off:])
	off += compression.CompressedSize(buf[off:])

	postingsLengths := compression.Decompress(buf[off:])

	postingsOffsets := make([]uint32, len(offsetDeltas))
	prev := uint32(0)
	for i, d := range offsetDeltas {
		prev += d
		postingsOffsets[i] = prev
	}

	entries := make([]TermEntry, len(terms))
	for i := range terms {
		entries[i] = TermEntry{
			Term:           terms[i],
			DocFreq:        docFreqs[i],
			PostingsOffset: header.PostingsOffset + int64(postingsOffsets[i]),
			PostingsLength: int32(postingsLengths[i]),
		}
	}

	return entries, nil
}

func ReadPostingsData(s *MMapStorage, offset int64, length int32) []byte {
	return s.Slice(offset, int(length))
}

func ReadDocLengths(s *MMapStorage, header *Header) ([]uint32, error) {
	offset := header.PostingsOffset - int64(header.DocLengthsSize)
	buf := s.Slice(offset, int(header.DocLengthsSize))
	return compression.Decompress(buf), nil
}
