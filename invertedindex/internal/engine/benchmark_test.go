package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/montanaflynn/stats"
	"github.com/pstpn/iidx/internal/index"
)

var (
	defaultBenchmarkSizes = []int{10000, 100000, 1000000}
	defaultBuildSizes     = []int{500, 700, 1000, 2000, 3000, 4000}
)

type wikiDoc struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

var (
	allDocs     []wikiDoc
	allDocsOnce sync.Once
	allDocsErr  error
)

func loadAllDocuments(b testing.TB) []wikiDoc {
	b.Helper()
	allDocsOnce.Do(func() {
		allDocs, allDocsErr = loadDocumentsFromFiles(2_000_000)
	})
	if allDocsErr != nil {
		b.Skipf("load documents: %v", allDocsErr)
	}
	return allDocs
}

func loadDocumentsFromFiles(maxCount int) ([]wikiDoc, error) {
	docsDir := filepath.Join("..", "..", "data", "documents")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return nil, fmt.Errorf("read documents dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	docs := make([]wikiDoc, 0, maxCount)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		f, err := os.Open(filepath.Join(docsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", entry.Name(), err)
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			var doc wikiDoc
			if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
				continue
			}
			docs = append(docs, doc)
			if len(docs) >= maxCount {
				f.Close()
				return docs, nil
			}
		}
		f.Close()

		if len(docs) >= maxCount {
			return docs, nil
		}
	}

	return docs, nil
}

func benchmarkSizes(b testing.TB) []int {
	b.Helper()
	return parseSizesEnv(b, "SIZES", defaultBenchmarkSizes)
}

func buildSizes(b testing.TB) []int {
	b.Helper()
	return parseSizesEnv(b, "BUILD_SIZES", defaultBuildSizes)
}

func parseSizesEnv(b testing.TB, envKey string, defaults []int) []int {
	b.Helper()

	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return defaults
	}

	parts := strings.Split(raw, ",")
	sizes := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		size, err := strconv.Atoi(value)
		if err != nil {
			b.Fatalf("invalid %s value %q: %v", envKey, value, err)
		}
		if size <= 0 {
			b.Fatalf("invalid %s value %q: must be positive", envKey, value)
		}
		sizes = append(sizes, size)
	}

	if len(sizes) == 0 {
		b.Fatalf("%s=%q did not contain any valid values", envKey, raw)
	}
	return sizes
}

func ci95(values []float64) float64 {
	n := len(values)
	if n <= 1 {
		return 0
	}
	sd, _ := stats.StandardDeviation(values)
	return 1.96 * sd / math.Sqrt(float64(n))
}

func buildIndex(b testing.TB, docs []wikiDoc, indexFile string) {
	b.Helper()

	builder := index.NewIndexBuilder()
	for _, doc := range docs {
		builder.AddDocument(doc.Title, doc.Text)
	}
	if err := builder.Save(indexFile); err != nil {
		b.Fatalf("save index: %v", err)
	}
}

func BenchmarkBuild(b *testing.B) {
	allDocs := loadAllDocuments(b)

	for _, size := range buildSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			if len(allDocs) < size {
				b.Skipf("not enough documents: got %d, need %d", len(allDocs), size)
			}
			docs := allDocs[:size]
			baseDir := b.TempDir()
			indexFile := filepath.Join(baseDir, fmt.Sprintf("build-%d.idx", size))
			docStoreFile := index.DocStoreFilename(indexFile)

			b.ReportAllocs()
			nsOpSamples := make([]float64, 0, 64)

			for b.Loop() {
				b.StopTimer()
				os.Remove(indexFile)
				os.Remove(docStoreFile)
				b.StartTimer()

				start := b.Elapsed()
				buildIndex(b, docs, indexFile)
				elapsed := b.Elapsed() - start

				nsOpSamples = append(nsOpSamples, float64(elapsed.Nanoseconds()))
			}

			b.StopTimer()
			b.ReportMetric(ci95(nsOpSamples), "ci95_ns/op")

			idxInfo, _ := os.Stat(indexFile)
			dsInfo, _ := os.Stat(docStoreFile)
			if idxInfo != nil && dsInfo != nil {
				totalBytes := float64(idxInfo.Size() + dsInfo.Size())
				b.ReportMetric(totalBytes, "bytes/index")
			}
		})
	}
}

func BenchmarkWarmup(b *testing.B) {
	allDocs := loadAllDocuments(b)

	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			if len(allDocs) < size {
				b.Skipf("not enough documents: got %d, need %d", len(allDocs), size)
			}
			docs := allDocs[:size]
			baseDir := b.TempDir()
			indexFile := filepath.Join(baseDir, fmt.Sprintf("warmup-%d.idx", size))

			b.StopTimer()
			buildIndex(b, docs, indexFile)
			b.StartTimer()

			b.ReportAllocs()
			warmupSamples := make([]float64, 0, 64)

			for b.Loop() {
				b.StopTimer()
				eng, err := LoadEngine(indexFile)
				if err != nil {
					b.Fatalf("load engine: %v", err)
				}
				b.StartTimer()

				start := b.Elapsed()
				_, err = eng.Search("the OR of OR to OR be OR in OR a OR for OR on OR it OR as OR with OR but")
				elapsed := b.Elapsed() - start
				if err != nil {
					b.Fatalf("search: %v", err)
				}

				warmupSamples = append(warmupSamples, float64(elapsed.Nanoseconds()))

				b.StopTimer()
				eng.Close()
				b.StartTimer()
			}

			b.StopTimer()
			b.ReportMetric(ci95(warmupSamples), "ci95_ns/warmup")
		})
	}
}

var searchQueries = []struct {
	name  string
	query string
}{
	{"Term", "the"},
	{"And", "the AND to"},
	{"Or", "the OR to"},
	{"Not", "the NOT to"},
	{"Adj", "to ADJ the"},
	{"Near", "the NEAR/3 to"},
	{"Complex_AndOr", "(the OR to) AND but"},
	{"Complex_AdjAnd", "to ADJ the AND but"},
	{"Complex_AndNot", "(the AND to) NOT but"},
}

func BenchmarkSearch(b *testing.B) {
	allDocs := loadAllDocuments(b)

	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			if len(allDocs) < size {
				b.Skipf("not enough documents: got %d, need %d", len(allDocs), size)
			}
			docs := allDocs[:size]
			baseDir := b.TempDir()
			indexFile := filepath.Join(baseDir, fmt.Sprintf("search-%d.idx", size))

			b.StopTimer()
			buildIndex(b, docs, indexFile)
			eng, err := LoadEngine(indexFile)
			if err != nil {
				b.Fatalf("load engine: %v", err)
			}
			b.Cleanup(func() {
				eng.Close()
			})
			if _, err := eng.Search("the OR of OR to OR be OR in OR a"); err != nil {
				b.Fatalf("warmup search: %v", err)
			}
			b.StartTimer()

			for _, sq := range searchQueries {
				b.Run(sq.name, func(b *testing.B) {
					b.ReportAllocs()
					nsQuerySamples := make([]float64, 0, 64)

					for b.Loop() {
						start := b.Elapsed()
						_, err := eng.Search(sq.query)
						elapsed := b.Elapsed() - start
						if err != nil {
							b.Fatalf("search %q: %v", sq.query, err)
						}

						nsQuerySamples = append(nsQuerySamples, float64(elapsed.Nanoseconds()))
					}

					b.ReportMetric(ci95(nsQuerySamples), "ci95_ns/query")
				})
			}
		})
	}
}
