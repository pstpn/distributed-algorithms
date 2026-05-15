package concurrent

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
)

const (
	defaultBuckets = 16
	defaultMaxLoad = 0.75
)

type Pair struct {
	Key   string
	Value string
}

type node struct {
	key   string
	value atomic.Pointer[string]
	next  atomic.Pointer[node]
}

type bucket struct {
	head atomic.Pointer[node]
}

type table struct {
	buckets []bucket
}

type Map struct {
	resizeMu      sync.RWMutex
	tbl           atomic.Pointer[table]
	size          atomic.Int64
	maxLoadFactor float64
}

func NewMap() *Map {
	return NewMapWithBuckets(defaultBuckets)
}

func NewMapWithBuckets(bucketCount int) *Map {
	if bucketCount < 1 {
		bucketCount = defaultBuckets
	}
	m := &Map{maxLoadFactor: defaultMaxLoad}
	m.tbl.Store(&table{buckets: make([]bucket, bucketCount)})
	return m
}

func (m *Map) Size() int {
	return int(m.size.Load())
}

func (m *Map) Get(key string) (string, bool) {
	tbl := m.tbl.Load()
	b := &tbl.buckets[hashIndex(key, len(tbl.buckets))]
	for n := b.head.Load(); n != nil; n = n.next.Load() {
		if n.key == key {
			if v := n.value.Load(); v != nil {
				return *v, true
			}
			return "", false
		}
	}
	return "", false
}

func (m *Map) Put(key string, value string) {
	m.resizeMu.RLock()
	tbl := m.tbl.Load()
	b := &tbl.buckets[hashIndex(key, len(tbl.buckets))]
	for {
		for n := b.head.Load(); n != nil; n = n.next.Load() {
			if n.key == key {
				for {
					oldVal := n.value.Load()
					if n.value.CompareAndSwap(oldVal, &value) {
						m.resizeMu.RUnlock()
						m.maybeResize()
						return
					}
				}
			}
		}
		newNode := &node{key: key}
		newNode.value.Store(&value)
		head := b.head.Load()
		newNode.next.Store(head)
		if b.head.CompareAndSwap(head, newNode) {
			m.size.Add(1)
			m.resizeMu.RUnlock()
			m.maybeResize()
			return
		}
	}
}

func (m *Map) Merge(key string, value string, merger func(string, string) string) string {
	m.resizeMu.RLock()
	tbl := m.tbl.Load()
	b := &tbl.buckets[hashIndex(key, len(tbl.buckets))]
	for {
		for n := b.head.Load(); n != nil; n = n.next.Load() {
			if n.key == key {
				for {
					oldVal := n.value.Load()
					if oldVal == nil {
						break
					}
					merged := merger(*oldVal, value)
					if n.value.CompareAndSwap(oldVal, &merged) {
						m.resizeMu.RUnlock()
						m.maybeResize()
						return merged
					}
				}
				break
			}
		}
		newNode := &node{key: key}
		newNode.value.Store(&value)
		head := b.head.Load()
		newNode.next.Store(head)
		if b.head.CompareAndSwap(head, newNode) {
			m.size.Add(1)
			m.resizeMu.RUnlock()
			m.maybeResize()
			return value
		}
	}
}

func (m *Map) Clear() {
	m.resizeMu.Lock()
	old := m.tbl.Load()
	m.tbl.Store(&table{buckets: make([]bucket, len(old.buckets))})
	m.size.Store(0)
	m.resizeMu.Unlock()
}

func (m *Map) Iterator() <-chan Pair {
	tbl := m.tbl.Load()
	pairs := make([]Pair, 0)
	for i := range tbl.buckets {
		for n := tbl.buckets[i].head.Load(); n != nil; n = n.next.Load() {
			if v := n.value.Load(); v != nil {
				pairs = append(pairs, Pair{Key: n.key, Value: *v})
			}
		}
	}
	ch := make(chan Pair, len(pairs))
	for _, p := range pairs {
		ch <- p
	}
	close(ch)
	return ch
}

func (m *Map) maybeResize() {
	tbl := m.tbl.Load()
	if float64(m.size.Load()) <= float64(len(tbl.buckets))*m.maxLoadFactor {
		return
	}
	m.resizeMu.Lock()
	defer m.resizeMu.Unlock()
	tbl = m.tbl.Load()
	if float64(m.size.Load()) <= float64(len(tbl.buckets))*m.maxLoadFactor {
		return
	}
	newSize := len(tbl.buckets) * 2
	if newSize < 1 {
		newSize = 1
	}
	newTable := &table{buckets: make([]bucket, newSize)}
	for i := range tbl.buckets {
		for n := tbl.buckets[i].head.Load(); n != nil; n = n.next.Load() {
			if v := n.value.Load(); v != nil {
				idx := hashIndex(n.key, len(newTable.buckets))
				nn := &node{key: n.key}
				nn.value.Store(v)
				head := newTable.buckets[idx].head.Load()
				nn.next.Store(head)
				newTable.buckets[idx].head.Store(nn)
			}
		}
	}
	m.tbl.Store(newTable)
}

func hashIndex(key string, bucketCount int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(bucketCount))
}
