package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	version    uint32 = 1
	magic             = "INVX"
	headerSize        = 40
)

type Header struct {
	Magic          [4]byte
	Version        uint32
	NumTerms       uint32
	NumDocs        uint32
	TotalTokens    uint64
	IndexOffset    int64
	PostingsOffset int64
}

func writeHeader(w io.Writer, h *Header) error {
	buf := make([]byte, headerSize)
	copy(buf[0:4], h.Magic[:])
	le := binary.LittleEndian
	le.PutUint32(buf[4:8], h.Version)
	le.PutUint32(buf[8:12], h.NumTerms)
	le.PutUint32(buf[12:16], h.NumDocs)
	le.PutUint64(buf[16:24], h.TotalTokens)
	le.PutUint64(buf[24:32], uint64(h.IndexOffset))
	le.PutUint64(buf[32:40], uint64(h.PostingsOffset))
	_, err := w.Write(buf)
	return err
}

func ReadHeader(s *MMapStorage) (*Header, error) {
	buf := make([]byte, headerSize)
	if _, err := s.Read(0, buf); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	h := &Header{}
	copy(h.Magic[:], buf[0:4])
	le := binary.LittleEndian
	h.Version = le.Uint32(buf[4:8])
	h.NumTerms = le.Uint32(buf[8:12])
	h.NumDocs = le.Uint32(buf[12:16])
	h.TotalTokens = le.Uint64(buf[16:24])
	h.IndexOffset = int64(le.Uint64(buf[24:32]))
	h.PostingsOffset = int64(le.Uint64(buf[32:40]))

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
	SkipListLength int32
}

func writeTermEntry(w io.Writer, e *TermEntry) error {
	le := binary.LittleEndian
	buf := make([]byte, 2+4+8+4+4+len(e.Term))

	le.PutUint16(buf[0:2], uint16(len(e.Term)))
	copy(buf[2:], e.Term)
	offset := 2 + len(e.Term)
	le.PutUint32(buf[offset:offset+4], e.DocFreq)
	le.PutUint64(buf[offset+4:offset+12], uint64(e.PostingsOffset))
	le.PutUint32(buf[offset+12:offset+16], uint32(e.PostingsLength))
	le.PutUint32(buf[offset+16:offset+20], uint32(e.SkipListLength))

	_, err := w.Write(buf[:offset+20])
	return err
}

func ReadTermEntries(s *MMapStorage, offset int64, count uint32) ([]TermEntry, error) {
	entries := make([]TermEntry, 0, count)
	currentOffset := offset

	for i := uint32(0); i < count; i++ {
		var termLenBuf [2]byte
		if _, err := s.Read(currentOffset, termLenBuf[:]); err != nil {
			return nil, fmt.Errorf("read term length at offset %d: %w", currentOffset, err)
		}
		le := binary.LittleEndian
		termLen := le.Uint16(termLenBuf[:])
		currentOffset += 2

		termBuf := make([]byte, termLen)
		if _, err := s.Read(currentOffset, termBuf); err != nil {
			return nil, fmt.Errorf("read term at offset %d: %w", currentOffset, err)
		}
		currentOffset += int64(termLen)

		var fields [20]byte
		if _, err := s.Read(currentOffset, fields[:]); err != nil {
			return nil, fmt.Errorf("read term fields at offset %d: %w", currentOffset, err)
		}

		entry := TermEntry{
			Term:           string(termBuf),
			DocFreq:        le.Uint32(fields[0:4]),
			PostingsOffset: int64(le.Uint64(fields[4:12])),
			PostingsLength: int32(le.Uint32(fields[12:16])),
			SkipListLength: int32(le.Uint32(fields[16:20])),
		}
		currentOffset += 20

		entries = append(entries, entry)
	}

	return entries, nil
}

func ReadPostingsData(s *MMapStorage, offset int64, length int32) ([]byte, error) {
	buf := make([]byte, length)
	if _, err := s.Read(offset, buf); err != nil {
		return nil, fmt.Errorf("read postings at offset %d: %w", offset, err)
	}
	return buf, nil
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
	skipLists   [][]byte
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
			Version:     version,
			NumDocs:     numDocs,
			TotalTokens: totalTokens,
		},
	}, nil
}

func (w *IndexWriter) AddTerm(term string, docFreq uint32, compressedPostings []byte, skipListData []byte) {
	w.termEntries = append(w.termEntries, TermEntry{
		Term:    term,
		DocFreq: docFreq,
	})
	w.postings = append(w.postings, compressedPostings)
	w.skipLists = append(w.skipLists, skipListData)
	w.header.NumTerms++
}

func (w *IndexWriter) Write() error {
	defer w.file.Close()

	le := binary.LittleEndian

	copy(w.header.Magic[:], magic)
	w.header.IndexOffset = headerSize

	if _, err := w.file.Seek(headerSize, io.SeekStart); err != nil {
		return fmt.Errorf("seek past header: %w", err)
	}

	bw := bufio.NewWriterSize(w.file, 256*1024)

	termIndexSize := int64(0)
	for _, entry := range w.termEntries {
		termIndexSize += int64(2 + len(entry.Term) + 4 + 8 + 4 + 4)
	}

	docLengthsSize := int64(w.header.NumDocs) * 4

	postingsOffset := headerSize + termIndexSize + docLengthsSize

	currentPostingsOffset := postingsOffset
	for i, postingData := range w.postings {
		skipListData := w.skipLists[i]
		w.termEntries[i].PostingsOffset = currentPostingsOffset
		w.termEntries[i].PostingsLength = int32(len(postingData))
		w.termEntries[i].SkipListLength = int32(len(skipListData))
		currentPostingsOffset += int64(len(skipListData)) + int64(len(postingData))
	}

	for _, entry := range w.termEntries {
		if err := writeTermEntry(bw, &entry); err != nil {
			return fmt.Errorf("write term entry %q: %w", entry.Term, err)
		}
	}

	docLengthsBuf := make([]byte, w.header.NumDocs*4)
	for i, dl := range w.docLengths {
		le.PutUint32(docLengthsBuf[i*4:], dl)
	}
	if _, err := bw.Write(docLengthsBuf); err != nil {
		return fmt.Errorf("write doc lengths: %w", err)
	}

	w.header.PostingsOffset = postingsOffset
	for i, postingData := range w.postings {
		if len(w.skipLists[i]) > 0 {
			if _, err := bw.Write(w.skipLists[i]); err != nil {
				return fmt.Errorf("write skip list for term %q: %w", w.termEntries[i].Term, err)
			}
		}
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

func ReadDocLengths(s *MMapStorage, header *Header) ([]uint32, error) {
	offset := header.PostingsOffset - int64(header.NumDocs)*4

	lengths := make([]uint32, header.NumDocs)
	buf := make([]byte, 4)
	le := binary.LittleEndian
	for i := uint32(0); i < header.NumDocs; i++ {
		if _, err := s.Read(offset+int64(i)*4, buf); err != nil {
			return nil, fmt.Errorf("read doc length %d: %w", i, err)
		}
		lengths[i] = le.Uint32(buf)
	}

	return lengths, nil
}
