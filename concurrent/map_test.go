package concurrent

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

func TestMapPutGetSize(t *testing.T) {
	m := NewMapWithBuckets(2)
	m.Put("a", "1")
	m.Put("b", "2")
	if m.Size() != 2 {
		t.Fatalf("expected size 2, got %d", m.Size())
	}
	v, ok := m.Get("a")
	if !ok || v != "1" {
		t.Fatalf("expected a=1, got %q, ok=%v", v, ok)
	}
	v, ok = m.Get("missing")
	if ok {
		t.Fatalf("expected missing key to be absent, got %q", v)
	}
}

func TestMapMerge(t *testing.T) {
	m := NewMapWithBuckets(2)
	out := m.Merge("k", "1", func(a, b string) string { return a + b })
	if out != "1" {
		t.Fatalf("expected merged value 1, got %q", out)
	}
	out = m.Merge("k", "2", func(a, b string) string { return a + b })
	if out != "12" {
		t.Fatalf("expected merged value 12, got %q", out)
	}
	v, ok := m.Get("k")
	if !ok || v != "12" {
		t.Fatalf("expected k=12, got %q, ok=%v", v, ok)
	}
}

func TestMapClear(t *testing.T) {
	m := NewMapWithBuckets(2)
	m.Put("a", "1")
	m.Put("b", "2")
	m.Clear()
	if m.Size() != 0 {
		t.Fatalf("expected size 0 after clear, got %d", m.Size())
	}
	if _, ok := m.Get("a"); ok {
		t.Fatalf("expected key to be absent after clear")
	}
}

func TestMapResize(t *testing.T) {
	m := NewMapWithBuckets(1)
	for i := 0; i < 1000; i++ {
		m.Put(fmt.Sprintf("k%d", i), "v")
	}
	if m.Size() != 1000 {
		t.Fatalf("expected size 1000 after resize, got %d", m.Size())
	}
	v, ok := m.Get("k999")
	if !ok || v != "v" {
		t.Fatalf("expected k999=v after resize, got %q, ok=%v", v, ok)
	}
}

func TestMapConcurrentAccess(t *testing.T) {
	m := NewMapWithBuckets(16)
	workers := runtime.GOMAXPROCS(0)
	keysPerWorker := 1000
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			start := id * keysPerWorker
			for i := 0; i < keysPerWorker; i++ {
				key := "k" + strconv.Itoa(start+i)
				m.Put(key, "v")
				m.Merge(key, "x", func(a, b string) string { return a + b })
				_, _ = m.Get(key)
				for range m.Iterator() {
				}
			}
		}(w)
	}
	wg.Wait()
	expected := workers * keysPerWorker
	if m.Size() != expected {
		t.Fatalf("expected size %d after concurrent writes, got %d", expected, m.Size())
	}
}
