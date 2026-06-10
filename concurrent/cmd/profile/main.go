package main

import (
	"concurrent"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"sync/atomic"
)

func main() {
	mode := flag.String("mode", "parallel", "execution mode: single or parallel")
	op := flag.String("op", "put", "operation to profile: put or get")
	size := flag.Int("size", 10000, "number of keys")
	rounds := flag.Int("rounds", 25, "number of rounds")
	procs := flag.Int("procs", 0, "GOMAXPROCS override (0 = keep current)")
	workers := flag.Int("workers", 0, "number of parallel workers (0 = GOMAXPROCS)")
	buckets := flag.Int("buckets", 0, "bucket count override (0 = default)")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	flag.Parse()

	if *cpuprofile == "" {
		fmt.Fprintln(os.Stderr, "error: -cpuprofile is required")
		os.Exit(1)
	}

	if *procs > 0 {
		runtime.GOMAXPROCS(*procs)
	}

	if *workers <= 0 {
		*workers = runtime.GOMAXPROCS(0)
	}

	f, err := os.Create(*cpuprofile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating profile file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "error starting cpu profile: %v\n", err)
		os.Exit(1)
	}
	defer pprof.StopCPUProfile()

	keys := makeKeys(*size)

	var m *concurrent.Map
	if *buckets > 0 {
		m = concurrent.NewMapWithBuckets(*buckets)
	} else {
		m = concurrent.NewMap()
	}

	if *op == "get" {
		fillMap(m, keys)
	}

	for r := 0; r < *rounds; r++ {
		switch *op {
		case "put":
			if *mode == "single" {
				for _, k := range keys {
					m.Put(k, "v")
				}
			} else {
				runParallel(len(keys), *workers, func(i int) {
					m.Put(keys[i], "v")
				})
			}
		case "get":
			if *mode == "single" {
				for _, k := range keys {
					m.Get(k)
				}
			} else {
				runParallel(len(keys), *workers, func(i int) {
					m.Get(keys[i])
				})
			}
		default:
			fmt.Fprintf(os.Stderr, "unknown op: %s\n", *op)
			os.Exit(1)
		}
	}
}

func makeKeys(size int) []string {
	keys := make([]string, size)
	for i := 0; i < size; i++ {
		keys[i] = "k" + strconv.Itoa(i)
	}
	return keys
}

func fillMap(m *concurrent.Map, keys []string) {
	for i, k := range keys {
		m.Put(k, "v"+strconv.Itoa(i))
	}
}

func runParallel(totalOps int, workers int, fn func(int)) {
	var counter int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt64(&counter, 1)) - 1
				if i >= totalOps {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}
