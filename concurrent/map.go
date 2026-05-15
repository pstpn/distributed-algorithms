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

type entry struct {
	key   string
	value string
}

type bucket struct {
	mu      sync.RWMutex
	entries []entry
}

type table struct {
	buckets []bucket
}

type Map struct {
	resizeMu      sync.RWMutex
	table         atomic.Value
	size          int64
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
	m.table.Store(&table{buckets: make([]bucket, bucketCount)})
	return m
}

func (m *Map) Size() int {
	return int(atomic.LoadInt64(&m.size))
}

func (m *Map) Get(key string) (string, bool) {
	tbl := m.table.Load().(*table)
	b := &tbl.buckets[hashIndex(key, len(tbl.buckets))]
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, e := range b.entries {
		if e.key == key {
			return e.value, true
		}
	}
	return "", false
}

func (m *Map) Put(key string, value string) {
	m.resizeMu.RLock()
	tbl := m.table.Load().(*table)
	b := &tbl.buckets[hashIndex(key, len(tbl.buckets))]
	b.mu.Lock()
	updated := false
	for i := range b.entries {
		if b.entries[i].key == key {
			b.entries[i].value = value
			updated = true
			break
		}
	}
	if !updated {
		b.entries = append(b.entries, entry{key: key, value: value})
		atomic.AddInt64(&m.size, 1)
	}
	b.mu.Unlock()
	m.resizeMu.RUnlock()
	m.maybeResize()
}

func (m *Map) Merge(key string, value string, merger func(string, string) string) string {
	m.resizeMu.RLock()
	tbl := m.table.Load().(*table)
	b := &tbl.buckets[hashIndex(key, len(tbl.buckets))]
	b.mu.Lock()
	var out string
	updated := false
	for i := range b.entries {
		if b.entries[i].key == key {
			if merger != nil {
				out = merger(b.entries[i].value, value)
			} else {
				out = value
			}
			b.entries[i].value = out
			updated = true
			break
		}
	}
	if !updated {
		out = value
		b.entries = append(b.entries, entry{key: key, value: out})
		atomic.AddInt64(&m.size, 1)
	}
	b.mu.Unlock()
	m.resizeMu.RUnlock()
	m.maybeResize()
	return out
}

func (m *Map) Clear() {
	m.resizeMu.Lock()
	old := m.table.Load().(*table)
	m.table.Store(&table{buckets: make([]bucket, len(old.buckets))})
	atomic.StoreInt64(&m.size, 0)
	m.resizeMu.Unlock()
}

func (m *Map) Iterator() <-chan Pair {
	tbl := m.table.Load().(*table)
	pairs := make([]Pair, 0)
	for i := range tbl.buckets {
		b := &tbl.buckets[i]
		b.mu.RLock()
		for _, e := range b.entries {
			pairs = append(pairs, Pair{Key: e.key, Value: e.value})
		}
		b.mu.RUnlock()
	}
	ch := make(chan Pair, len(pairs))
	for _, p := range pairs {
		ch <- p
	}
	close(ch)
	return ch
}

func (m *Map) maybeResize() {
	tbl := m.table.Load().(*table)
	if float64(atomic.LoadInt64(&m.size)) <= float64(len(tbl.buckets))*m.maxLoadFactor {
		return
	}
	m.resizeMu.Lock()
	defer m.resizeMu.Unlock()
	tbl = m.table.Load().(*table)
	if float64(atomic.LoadInt64(&m.size)) <= float64(len(tbl.buckets))*m.maxLoadFactor {
		return
	}
	newSize := len(tbl.buckets) * 2
	if newSize < 1 {
		newSize = 1
	}
	newTable := &table{buckets: make([]bucket, newSize)}
	for i := range tbl.buckets {
		b := &tbl.buckets[i]
		b.mu.RLock()
		for _, e := range b.entries {
			idx := hashIndex(e.key, len(newTable.buckets))
			newTable.buckets[idx].entries = append(newTable.buckets[idx].entries, e)
		}
		b.mu.RUnlock()
	}
	m.table.Store(newTable)
}

func hashIndex(key string, bucketCount int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(bucketCount))
}
