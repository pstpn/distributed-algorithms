package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	docStoreMagic      = "DOCS"
	docStoreHeaderSize = 16
)

type DocStoreHeader struct {
	Magic   [4]byte
	Version uint32
	NumDocs uint32
	_       uint32
}

type DocStoreWriter struct {
	file       *os.File
	header     DocStoreHeader
	docEntries []DocEntry
}

func NewDocStoreWriter(filename string, numDocs uint32, docEntries []DocEntry) (*DocStoreWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create doc store file %s: %w", filename, err)
	}

	return &DocStoreWriter{
		file:       file,
		docEntries: docEntries,
		header: DocStoreHeader{
			Version: 1,
			NumDocs: numDocs,
		},
	}, nil
}

func (w *DocStoreWriter) Write() error {
	defer w.file.Close()

	copy(w.header.Magic[:], docStoreMagic)
	le := binary.LittleEndian

	if _, err := w.file.Seek(docStoreHeaderSize, 0); err != nil {
		return fmt.Errorf("seek past header: %w", err)
	}

	bw := bufio.NewWriterSize(w.file, 256*1024)

	offsetTableStart := int64(docStoreHeaderSize)
	docDataStart := offsetTableStart + int64(w.header.NumDocs)*8

	currentOffset := docDataStart
	docOffsets := make([]int64, len(w.docEntries))
	for i, doc := range w.docEntries {
		docOffsets[i] = currentOffset
		currentOffset += int64(4 + len(doc.Title) + 4 + len(doc.Text))
	}

	var offBuf [8]byte
	for _, off := range docOffsets {
		le.PutUint64(offBuf[:], uint64(off))
		if _, err := bw.Write(offBuf[:]); err != nil {
			return fmt.Errorf("write doc offset: %w", err)
		}
	}

	var lenBuf [4]byte
	for _, doc := range w.docEntries {
		titleBytes := []byte(doc.Title)
		le.PutUint32(lenBuf[:], uint32(len(titleBytes)))
		if _, err := bw.Write(lenBuf[:]); err != nil {
			return fmt.Errorf("write doc title length: %w", err)
		}
		if _, err := bw.Write(titleBytes); err != nil {
			return fmt.Errorf("write doc title: %w", err)
		}

		textBytes := []byte(doc.Text)
		le.PutUint32(lenBuf[:], uint32(len(textBytes)))
		if _, err := bw.Write(lenBuf[:]); err != nil {
			return fmt.Errorf("write doc text length: %w", err)
		}
		if _, err := bw.Write(textBytes); err != nil {
			return fmt.Errorf("write doc text: %w", err)
		}
	}

	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush buffer: %w", err)
	}

	if _, err := w.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek to header: %w", err)
	}

	headerBuf := make([]byte, docStoreHeaderSize)
	copy(headerBuf[0:4], w.header.Magic[:])
	le.PutUint32(headerBuf[4:8], w.header.Version)
	le.PutUint32(headerBuf[8:12], w.header.NumDocs)
	le.PutUint32(headerBuf[12:16], 0)

	if _, err := w.file.Write(headerBuf); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	return w.file.Sync()
}

type DocStore struct {
	storage *MMapStorage
	header  DocStoreHeader
	numDocs uint32
}

func OpenDocStore(filename string) (*DocStore, error) {
	storage, err := OpenMMap(filename)
	if err != nil {
		return nil, fmt.Errorf("open mmap: %w", err)
	}

	buf := storage.Slice(0, docStoreHeaderSize)

	h := DocStoreHeader{}
	copy(h.Magic[:], buf[0:4])
	le := binary.LittleEndian
	h.Version = le.Uint32(buf[4:8])
	h.NumDocs = le.Uint32(buf[8:12])

	if string(h.Magic[:]) != docStoreMagic {
		storage.Close()
		return nil, fmt.Errorf("invalid magic: expected %q, got %q", docStoreMagic, h.Magic[:])
	}

	return &DocStore{
		storage: storage,
		header:  h,
		numDocs: h.NumDocs,
	}, nil
}

func (ds *DocStore) GetDocumentTitle(docID uint32) (string, error) {
	if docID == 0 || docID > ds.numDocs {
		return "", fmt.Errorf("doc id %d out of range [1, %d]", docID, ds.numDocs)
	}

	offset, err := ds.readDocOffset(docID)
	if err != nil {
		return "", err
	}

	title, _, err := ds.readDocData(offset)
	if err != nil {
		return "", err
	}
	return title, nil
}

func (ds *DocStore) GetDocumentText(docID uint32) (string, error) {
	if docID == 0 || docID > ds.numDocs {
		return "", fmt.Errorf("doc id %d out of range [1, %d]", docID, ds.numDocs)
	}

	offset, err := ds.readDocOffset(docID)
	if err != nil {
		return "", err
	}

	_, text, err := ds.readDocData(offset)
	if err != nil {
		return "", err
	}
	return text, nil
}

func (ds *DocStore) GetDocument(docID uint32) (title string, text string, err error) {
	if docID == 0 || docID > ds.numDocs {
		return "", "", fmt.Errorf("doc id %d out of range [1, %d]", docID, ds.numDocs)
	}

	offset, err := ds.readDocOffset(docID)
	if err != nil {
		return "", "", err
	}

	return ds.readDocData(offset)
}

func (ds *DocStore) NumDocs() uint32 {
	return ds.numDocs
}

func (ds *DocStore) Close() error {
	if ds.storage != nil {
		return ds.storage.Close()
	}
	return nil
}

func (ds *DocStore) readDocOffset(docID uint32) (int64, error) {
	tableOffset := int64(docStoreHeaderSize) + int64(docID-1)*8
	offBuf := ds.storage.Slice(tableOffset, 8)
	return int64(binary.LittleEndian.Uint64(offBuf)), nil
}

func (ds *DocStore) readDocData(offset int64) (string, string, error) {
	le := binary.LittleEndian

	lenBuf := ds.storage.Slice(offset, 4)
	titleLen := le.Uint32(lenBuf)
	offset += 4

	var title string
	if titleLen > 0 {
		title = string(ds.storage.Slice(offset, int(titleLen)))
	}
	offset += int64(titleLen)

	lenBuf = ds.storage.Slice(offset, 4)
	textLen := le.Uint32(lenBuf)
	offset += 4

	var text string
	if textLen > 0 {
		text = string(ds.storage.Slice(offset, int(textLen)))
	}

	return title, text, nil
}
