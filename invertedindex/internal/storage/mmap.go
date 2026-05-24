package storage

import (
	"fmt"
	"os"

	"golang.org/x/exp/mmap"
)

type MMapStorage struct {
	file   *os.File
	reader *mmap.ReaderAt
	size   int64
}

func OpenMMap(filename string) (*MMapStorage, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", filename, err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat file %s: %w", filename, err)
	}

	reader, err := mmap.Open(filename)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("mmap file %s: %w", filename, err)
	}

	return &MMapStorage{
		file:   file,
		reader: reader,
		size:   info.Size(),
	}, nil
}

func (m *MMapStorage) Read(offset int64, p []byte) (int, error) {
	if offset >= m.size {
		return 0, fmt.Errorf("offset %d beyond file size %d", offset, m.size)
	}
	return m.reader.ReadAt(p, offset)
}

func (m *MMapStorage) Size() int64 {
	return m.size
}

func (m *MMapStorage) Close() error {
	var firstErr error
	if err := m.reader.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := m.file.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
