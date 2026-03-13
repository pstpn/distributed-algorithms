package extendible

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	ErrInvalidBucketCapacity = errors.New("extendible: bucket capacity must be positive")
	ErrKeyExists             = errors.New("extendible: key already exists")
	ErrKeyNotFound           = errors.New("extendible: key not found")
	ErrHashSpaceExhausted    = errors.New("extendible: cannot split bucket further")
	ErrInvalidFileFormat     = errors.New("extendible: invalid table file format")
)

const (
	magicValue      = uint64(0x4558544841534831) // EXTHASH1
	versionValue    = uint32(1)
	headerSize      = 4096
	directoryOffset = headerSize

	maxGlobalDepth = uint32(20)

	headerMagicOffset          = 0
	headerVersionOffset        = 8
	headerBucketCapacityOffset = 12
	headerGlobalDepthOffset    = 16
	headerNextBucketIDOffset   = 20
	headerBucketStrideOffset   = 24
	headerBucketBaseOffset     = 32
)

type diskEntry struct {
	Key   uint64
	Value uint64
	Hash  uint64
}

type Stats struct {
	Size           int
	BucketCount    int
	BucketCapacity int
	GlobalDepth    uint
	MaxBucketLoad  int
	FileBytes      int64
}

type Table struct {
	path           string
	file           *os.File
	mmap           []byte
	hashFunc       func(uint64) uint64
	bucketCapacity uint32
	bucketStride   uint64
	bucketBase     uint64
}

func NewTable(path string, bucketCapacity int, hashFunc func(uint64) uint64) (*Table, error) {
	if bucketCapacity <= 0 {
		return nil, ErrInvalidBucketCapacity
	}
	if bucketCapacity > (1 << 20) {
		return nil, errors.New("extendible: bucket capacity is too large")
	}
	if hashFunc == nil {
		return nil, errors.New("extendible: hash function is required")
	}
	if path == "" {
		return nil, errors.New("extendible: table file path is required")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}

	t := &Table{
		path:           path,
		file:           file,
		hashFunc:       hashFunc,
		bucketCapacity: uint32(bucketCapacity),
	}
	t.bucketStride = uint64(16 + int(t.bucketCapacity)*int(unsafe.Sizeof(diskEntry{})))
	t.bucketBase = uint64(directoryOffset) + uint64(t.maxDirectoryEntries())*4

	initialSize := int64(t.bucketBase + t.bucketStride)
	if err := file.Truncate(initialSize); err != nil {
		file.Close()
		return nil, err
	}
	if err := t.mapFile(initialSize); err != nil {
		file.Close()
		return nil, err
	}

	t.setUint64(headerMagicOffset, magicValue)
	t.setUint32(headerVersionOffset, versionValue)
	t.setUint32(headerBucketCapacityOffset, t.bucketCapacity)
	t.setUint32(headerGlobalDepthOffset, 0)
	t.setUint32(headerNextBucketIDOffset, 1)
	t.setUint64(headerBucketStrideOffset, t.bucketStride)
	t.setUint64(headerBucketBaseOffset, t.bucketBase)

	for i := uint32(0); i < t.maxDirectoryEntries(); i++ {
		t.setDirectory(i, 0)
	}
	t.setBucketLocalDepth(0, 0)
	t.setBucketCount(0, 0)

	return t, nil
}

func OpenTable(path string, hashFunc func(uint64) uint64) (*Table, error) {
	if hashFunc == nil {
		return nil, errors.New("extendible: hash function is required")
	}
	if path == "" {
		return nil, errors.New("extendible: table file path is required")
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	t := &Table{path: path, file: file, hashFunc: hashFunc}
	if err := t.mapFile(info.Size()); err != nil {
		file.Close()
		return nil, err
	}

	if t.uint64At(headerMagicOffset) != magicValue || t.uint32At(headerVersionOffset) != versionValue {
		t.Close()
		return nil, ErrInvalidFileFormat
	}
	t.bucketCapacity = t.uint32At(headerBucketCapacityOffset)
	t.bucketStride = t.uint64At(headerBucketStrideOffset)
	t.bucketBase = t.uint64At(headerBucketBaseOffset)
	if t.bucketCapacity == 0 || t.bucketStride < 16 {
		t.Close()
		return nil, ErrInvalidFileFormat
	}

	return t, nil
}

func (t *Table) Close() error {
	var errClose error
	if t.mmap != nil {
		if err := syscall.Munmap(t.mmap); err != nil {
			errClose = err
		}
		t.mmap = nil
	}
	if t.file != nil {
		if err := t.file.Close(); err != nil {
			errClose = errors.Join(errClose, err)
		}
		t.file = nil
	}
	return errClose
}

func (t *Table) Sync() error {
	return t.file.Sync()
}

func (t *Table) Insert(key uint64, value uint64) error {
	hash := t.hashFunc(key)
	for {
		index := t.directoryIndex(hash)
		bucketID := t.directoryAt(index)

		if entryIndex := t.findInBucket(bucketID, key); entryIndex >= 0 {
			return ErrKeyExists
		}

		count := t.bucketCount(bucketID)
		if count < t.bucketCapacity {
			t.writeEntry(bucketID, count, diskEntry{Key: key, Value: value, Hash: hash})
			t.setBucketCount(bucketID, count+1)
			return nil
		}

		if err := t.splitBucket(index, bucketID); err != nil {
			return err
		}
	}
}

func (t *Table) Update(key uint64, value uint64) error {
	bucketID := t.directoryAt(t.directoryIndex(t.hashFunc(key)))
	entryIndex := t.findInBucket(bucketID, key)
	if entryIndex < 0 {
		return ErrKeyNotFound
	}
	entry := t.readEntry(bucketID, uint32(entryIndex))
	entry.Value = value
	t.writeEntry(bucketID, uint32(entryIndex), entry)
	return nil
}

func (t *Table) Get(key uint64) (uint64, bool) {
	bucketID := t.directoryAt(t.directoryIndex(t.hashFunc(key)))
	entryIndex := t.findInBucket(bucketID, key)
	if entryIndex < 0 {
		return 0, false
	}
	return t.readEntry(bucketID, uint32(entryIndex)).Value, true
}

func (t *Table) Delete(key uint64) bool {
	index := t.directoryIndex(t.hashFunc(key))
	bucketID := t.directoryAt(index)
	entryIndex := t.findInBucket(bucketID, key)
	if entryIndex < 0 {
		return false
	}

	count := t.bucketCount(bucketID)
	last := count - 1
	if uint32(entryIndex) != last {
		t.writeEntry(bucketID, uint32(entryIndex), t.readEntry(bucketID, last))
	}
	t.setBucketCount(bucketID, last)

	t.tryMerge(index)
	t.tryShrinkDirectory()
	return true
}

func (t *Table) Len() int {
	total := 0
	for bucketID := uint32(0); bucketID < t.nextBucketID(); bucketID++ {
		total += int(t.bucketCount(bucketID))
	}
	return total
}

func (t *Table) Stats() Stats {
	unique := make(map[uint32]struct{}, 1<<t.globalDepth())
	total := 0
	maxBucketLoad := 0

	depth := t.globalDepth()
	limit := uint32(1) << depth
	for directoryIndex := uint32(0); directoryIndex < limit; directoryIndex++ {
		bucketID := t.directoryAt(directoryIndex)
		unique[bucketID] = struct{}{}
		count := int(t.bucketCount(bucketID))
		total += count
		if count > maxBucketLoad {
			maxBucketLoad = count
		}
	}

	var fileBytes int64
	if info, err := t.file.Stat(); err == nil {
		fileBytes = info.Size()
	}

	return Stats{
		Size:           total,
		BucketCount:    len(unique),
		BucketCapacity: int(t.bucketCapacity),
		GlobalDepth:    uint(depth),
		MaxBucketLoad:  maxBucketLoad,
		FileBytes:      fileBytes,
	}
}

func (t *Table) String() string {
	stats := t.Stats()
	return fmt.Sprintf("Table(path=%s, size=%d, buckets=%d, globalDepth=%d)", t.path, stats.Size, stats.BucketCount, stats.GlobalDepth)
}

func Uint64Hasher(key uint64) uint64 {
	return mix64(key)
}

func Uint32Hasher(key uint32) uint64 {
	return mix64(uint64(key))
}

func IntHasher(key int) uint64 {
	return mix64(uint64(key))
}

func StringHasher(key string) uint64 {
	const (
		offset64 = uint64(14695981039346656037)
		prime64  = uint64(1099511628211)
	)

	hash := offset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}
	return mix64(hash)
}

func (t *Table) splitBucket(_ uint32, targetBucketID uint32) error {
	localDepth := t.bucketLocalDepth(targetBucketID)
	if localDepth >= 63 {
		return ErrHashSpaceExhausted
	}

	if localDepth == t.globalDepth() {
		if err := t.doubleDirectory(); err != nil {
			return err
		}
	}

	newLocalDepth := localDepth + 1
	newBucketID, err := t.allocateBucket(newLocalDepth)
	if err != nil {
		return err
	}
	t.setBucketLocalDepth(targetBucketID, newLocalDepth)

	splitBit := uint32(1) << (newLocalDepth - 1)
	limit := uint32(1) << t.globalDepth()
	for directoryIndex := uint32(0); directoryIndex < limit; directoryIndex++ {
		if t.directoryAt(directoryIndex) != targetBucketID {
			continue
		}
		if directoryIndex&splitBit != 0 {
			t.setDirectory(directoryIndex, newBucketID)
		}
	}

	count := t.bucketCount(targetBucketID)
	existing := make([]diskEntry, count)
	for entryIndex := uint32(0); entryIndex < count; entryIndex++ {
		existing[entryIndex] = t.readEntry(targetBucketID, entryIndex)
	}
	t.setBucketCount(targetBucketID, 0)
	t.setBucketCount(newBucketID, 0)

	for _, item := range existing {
		targetID := t.directoryAt(t.directoryIndex(item.Hash))
		targetCount := t.bucketCount(targetID)
		t.writeEntry(targetID, targetCount, item)
		t.setBucketCount(targetID, targetCount+1)
	}

	return nil
}

func (t *Table) doubleDirectory() error {
	if t.globalDepth() >= maxGlobalDepth {
		return ErrHashSpaceExhausted
	}

	currentDepth := t.globalDepth()
	currentSize := uint32(1) << currentDepth
	for index := uint32(0); index < currentSize; index++ {
		t.setDirectory(index+currentSize, t.directoryAt(index))
	}
	t.setUint32(headerGlobalDepthOffset, currentDepth+1)
	return nil
}

func (t *Table) tryMerge(index uint32) {
	for {
		bucketID := t.directoryAt(index)
		localDepth := t.bucketLocalDepth(bucketID)
		if localDepth == 0 {
			return
		}

		splitBit := uint32(1) << (localDepth - 1)
		buddyIndex := index ^ splitBit
		buddyID := t.directoryAt(buddyIndex)
		if buddyID == bucketID || t.bucketLocalDepth(buddyID) != localDepth {
			return
		}
		if t.bucketCount(bucketID)+t.bucketCount(buddyID) > t.bucketCapacity {
			return
		}

		mergedIndex := index &^ splitBit
		mergedBucketID := t.directoryAt(mergedIndex)
		donorBucketID := t.directoryAt(mergedIndex | splitBit)
		if mergedBucketID == donorBucketID {
			return
		}

		donorCount := t.bucketCount(donorBucketID)
		mergedCount := t.bucketCount(mergedBucketID)
		for entryIndex := uint32(0); entryIndex < donorCount; entryIndex++ {
			t.writeEntry(mergedBucketID, mergedCount+entryIndex, t.readEntry(donorBucketID, entryIndex))
		}
		t.setBucketCount(mergedBucketID, mergedCount+donorCount)
		t.setBucketCount(donorBucketID, 0)
		t.setBucketLocalDepth(mergedBucketID, localDepth-1)

		limit := uint32(1) << t.globalDepth()
		for directoryIndex := uint32(0); directoryIndex < limit; directoryIndex++ {
			current := t.directoryAt(directoryIndex)
			if current == donorBucketID || current == mergedBucketID {
				t.setDirectory(directoryIndex, mergedBucketID)
			}
		}

		index = mergedIndex
	}
}

func (t *Table) tryShrinkDirectory() {
	for t.globalDepth() > 0 {
		half := (uint32(1) << t.globalDepth()) / 2
		shrinkable := true
		for index := uint32(0); index < half; index++ {
			if t.directoryAt(index) != t.directoryAt(index+half) {
				shrinkable = false
				break
			}
		}
		if !shrinkable {
			return
		}
		t.setUint32(headerGlobalDepthOffset, t.globalDepth()-1)
	}
}

func (t *Table) directoryIndex(hash uint64) uint32 {
	mask := depthMask(uint(t.globalDepth()))
	return uint32(hash & mask)
}

func (t *Table) findInBucket(bucketID uint32, key uint64) int {
	count := t.bucketCount(bucketID)
	for index := uint32(0); index < count; index++ {
		if t.readEntry(bucketID, index).Key == key {
			return int(index)
		}
	}
	return -1
}

func (t *Table) allocateBucket(localDepth uint32) (uint32, error) {
	id := t.nextBucketID()
	requiredSize := int64(t.bucketOffset(id + 1))
	if err := t.ensureSize(requiredSize); err != nil {
		return 0, err
	}
	t.setBucketLocalDepth(id, localDepth)
	t.setBucketCount(id, 0)
	t.setUint32(headerNextBucketIDOffset, id+1)
	return id, nil
}

func (t *Table) ensureSize(required int64) error {
	if int64(len(t.mmap)) >= required {
		return nil
	}
	newSize := int64(len(t.mmap))
	if newSize == 0 {
		newSize = 1 << 20
	}
	for newSize < required {
		newSize *= 2
	}
	return t.remap(newSize)
}

func (t *Table) mapFile(size int64) error {
	mapped, err := syscall.Mmap(int(t.file.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return err
	}
	t.mmap = mapped
	return nil
}

func (t *Table) remap(size int64) error {
	if t.mmap != nil {
		if err := syscall.Munmap(t.mmap); err != nil {
			return err
		}
		t.mmap = nil
	}
	if err := t.file.Truncate(size); err != nil {
		return err
	}
	return t.mapFile(size)
}

func (t *Table) globalDepth() uint32 {
	return t.uint32At(headerGlobalDepthOffset)
}

func (t *Table) nextBucketID() uint32 {
	return t.uint32At(headerNextBucketIDOffset)
}

func (t *Table) maxDirectoryEntries() uint32 {
	return uint32(1) << maxGlobalDepth
}

func (t *Table) directoryAt(index uint32) uint32 {
	return t.uint32At(directoryOffset + int(index*4))
}

func (t *Table) setDirectory(index uint32, bucketID uint32) {
	t.setUint32(directoryOffset+int(index*4), bucketID)
}

func (t *Table) bucketOffset(bucketID uint32) uint64 {
	return t.bucketBase + uint64(bucketID)*t.bucketStride
}

func (t *Table) bucketLocalDepth(bucketID uint32) uint32 {
	return t.uint32At(int(t.bucketOffset(bucketID)))
}

func (t *Table) setBucketLocalDepth(bucketID uint32, depth uint32) {
	t.setUint32(int(t.bucketOffset(bucketID)), depth)
}

func (t *Table) bucketCount(bucketID uint32) uint32 {
	return t.uint32At(int(t.bucketOffset(bucketID)) + 4)
}

func (t *Table) setBucketCount(bucketID uint32, count uint32) {
	t.setUint32(int(t.bucketOffset(bucketID))+4, count)
}

func (t *Table) entryOffset(bucketID uint32, entryIndex uint32) uint64 {
	return t.bucketOffset(bucketID) + 16 + uint64(entryIndex)*uint64(unsafe.Sizeof(diskEntry{}))
}

func (t *Table) readEntry(bucketID uint32, entryIndex uint32) diskEntry {
	ptr := (*diskEntry)(unsafe.Pointer(&t.mmap[t.entryOffsetInt(bucketID, entryIndex)]))
	return *ptr
}

func (t *Table) writeEntry(bucketID uint32, entryIndex uint32, value diskEntry) {
	ptr := (*diskEntry)(unsafe.Pointer(&t.mmap[t.entryOffsetInt(bucketID, entryIndex)]))
	*ptr = value
}

func (t *Table) entryOffsetInt(bucketID uint32, entryIndex uint32) int {
	return int(t.entryOffset(bucketID, entryIndex))
}

func (t *Table) uint32At(offset int) uint32 {
	return *(*uint32)(unsafe.Pointer(&t.mmap[offset]))
}

func (t *Table) setUint32(offset int, value uint32) {
	*(*uint32)(unsafe.Pointer(&t.mmap[offset])) = value
}

func (t *Table) uint64At(offset int) uint64 {
	return *(*uint64)(unsafe.Pointer(&t.mmap[offset]))
}

func (t *Table) setUint64(offset int, value uint64) {
	*(*uint64)(unsafe.Pointer(&t.mmap[offset])) = value
}

func depthMask(depth uint) uint64 {
	if depth == 0 {
		return 0
	}
	if depth >= 64 {
		return ^uint64(0)
	}
	return (uint64(1) << depth) - 1
}

func mix64(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}
