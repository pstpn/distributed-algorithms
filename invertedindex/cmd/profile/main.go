package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/pstpn/iidx/internal/engine"
	"github.com/pstpn/iidx/internal/index"
)

func main() {
	mode := flag.String("mode", "build", "profile mode: build or search")
	size := flag.Int("size", 10000, "number of documents for build profile")
	query := flag.String("query", "cat AND dog OR hello NEAR/5 world", "search query for search profile")
	rounds := flag.Int("rounds", 5, "number of search rounds")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	flag.Parse()

	if *cpuprofile == "" {
		fmt.Fprintln(os.Stderr, "error: -cpuprofile is required")
		os.Exit(1)
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

	switch *mode {
	case "build":
		profileBuild(*size)
	case "search":
		profileSearch(*query, *rounds)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func profileBuild(size int) {
	tmpDir, err := os.MkdirTemp("", "iidx-profile-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	b := index.NewIndexBuilder()
	for i := 0; i < size; i++ {
		title := "Document " + strconv.Itoa(i)
		text := generateText(i)
		b.AddDocument(title, text)
	}

	idxPath := tmpDir + "/profile.idx"
	if err := b.Save(idxPath); err != nil {
		fmt.Fprintf(os.Stderr, "save: %v\n", err)
		os.Exit(1)
	}

	eng, err := engine.LoadEngine(idxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(1)
	}
	eng.Close()

	fmt.Printf("build profile: %d documents indexed\n", size)
}

func profileSearch(queryStr string, rounds int) {
	tmpDir, err := os.MkdirTemp("", "iidx-profile-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	const docCount = 10000
	b := index.NewIndexBuilder()
	for i := 0; i < docCount; i++ {
		title := "Document " + strconv.Itoa(i)
		text := generateText(i)
		b.AddDocument(title, text)
	}

	idxPath := tmpDir + "/profile.idx"
	if err := b.Save(idxPath); err != nil {
		fmt.Fprintf(os.Stderr, "save: %v\n", err)
		os.Exit(1)
	}

	eng, err := engine.LoadEngine(idxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	for r := 0; r < rounds; r++ {
		result, err := eng.Search(queryStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "search: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("round %d: %d results\n", r+1, len(result.Docs))
	}
}

func generateText(seed int) string {
	words := []string{
		"cat", "dog", "hello", "world", "foo", "bar", "baz",
		"common", "rare", "unique", "alpha", "beta", "gamma",
		"delta", "epsilon", "zeta", "eta", "theta", "iota",
		"kappa", "lambda", "mu", "nu", "xi", "omicron", "pi",
	}

	var b strings.Builder
	n := 20 + seed%30
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		w := words[(seed+i*7+3)%len(words)]
		if i%5 == 0 {
			w = words[seed%5]
		}
		b.WriteString(w)
	}
	return b.String()
}
