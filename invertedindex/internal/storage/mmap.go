package storage

import (
	"fmt"
	"os"
	"syscall"
)

type MMapStorage struct {
	file *os.File
	data []byte
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

	size := info.Size()
	if size == 0 {
		return &MMapStorage{file: file}, nil
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("mmap file %s: %w", filename, err)
	}

	return &MMapStorage{
		file: file,
		data: data,
	}, nil
}

func (m *MMapStorage) Slice(offset int64, length int) []byte {
	return m.data[offset : offset+int64(length)]
}

func (m *MMapStorage) Size() int64 {
	return int64(len(m.data))
}

func (m *MMapStorage) Close() error {
	var firstErr error
	if m.data != nil {
		if err := syscall.Munmap(m.data); err != nil {
			firstErr = err
		}
	}
	if err := m.file.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
