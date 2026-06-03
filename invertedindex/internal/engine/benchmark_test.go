package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/montanaflynn/stats"
	"github.com/pstpn/iidx/internal/index"
	"github.com/pstpn/iidx/internal/storage"
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

func loadIndexTerms(indexFile string) []string {
	s, err := storage.OpenMMap(indexFile)
	if err != nil {
		return nil
	}
	defer s.Close()

	header, err := storage.ReadHeader(s)
	if err != nil {
		return nil
	}

	termEntries, err := storage.ReadTermEntries(s, header)
	if err != nil {
		return nil
	}

	keywords := map[string]bool{"AND": true, "OR": true, "NOT": true, "ADJ": true, "NEAR": true}

	filtered := make([]storage.TermEntry, 0, len(termEntries))
	for _, entry := range termEntries {
		if keywords[strings.ToUpper(entry.Term)] {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].DocFreq < filtered[j].DocFreq
	})

	cutoff := len(filtered) / 2
	terms := make([]string, cutoff)
	for i := 0; i < cutoff; i++ {
		terms[i] = filtered[i].Term
	}
	return terms
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
			if idxInfo != nil {
				b.ReportMetric(float64(idxInfo.Size()), "bytes/index")
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

type searchQueryType struct {
	name string
	gen  func(rng *rand.Rand, terms []string) string
}

func makeSearchQueryTypes() []searchQueryType {
	return []searchQueryType{
		{"Or", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))] + " OR " + terms[rng.Intn(len(terms))]
		}},
		{"And", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))] + " AND " + terms[rng.Intn(len(terms))]
		}},
		{"Term", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))]
		}},
		{"Not", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))] + " AND NOT " + terms[rng.Intn(len(terms))]
		}},
		{"Adj", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))] + " ADJ " + terms[rng.Intn(len(terms))]
		}},
		{"Near", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))] + " NEAR/3 " + terms[rng.Intn(len(terms))]
		}},
		{"Complex_AndOr", func(rng *rand.Rand, terms []string) string {
			return "(" + terms[rng.Intn(len(terms))] + " OR " + terms[rng.Intn(len(terms))] + ") AND " + terms[rng.Intn(len(terms))]
		}},
		{"Complex_AdjAnd", func(rng *rand.Rand, terms []string) string {
			return terms[rng.Intn(len(terms))] + " ADJ " + terms[rng.Intn(len(terms))] + " AND " + terms[rng.Intn(len(terms))]
		}},
		{"Complex_AndNot", func(rng *rand.Rand, terms []string) string {
			return "(" + terms[rng.Intn(len(terms))] + " AND " + terms[rng.Intn(len(terms))] + ") AND NOT " + terms[rng.Intn(len(terms))]
		}},
	}
}

const searchQueriesPerType = 10000

func benchmarkSearchEngine(b *testing.B, size int) (*Engine, []string) {
	b.Helper()

	allDocs := loadAllDocuments(b)
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
	if _, _, err := eng.Evaluate("the OR of OR to OR be OR in OR a"); err != nil {
		b.Fatalf("warmup evaluate: %v", err)
	}

	terms := loadIndexTerms(indexFile)
	if len(terms) == 0 {
		b.Fatal("no terms found in index")
	}
	b.StartTimer()

	return eng, terms
}

func BenchmarkSearch(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			eng, terms := benchmarkSearchEngine(b, size)

			for _, qt := range makeSearchQueryTypes() {
				b.Run(qt.name, func(b *testing.B) {
					rng := rand.New(rand.NewSource(42))
					queries := make([]string, searchQueriesPerType)
					for i := range queries {
						queries[i] = qt.gen(rng, terms)
					}

					nsQuerySamples := make([]float64, 0, searchQueriesPerType*10)

					for b.Loop() {
						for _, q := range queries {
							start := b.Elapsed()
							_, _, err := eng.Evaluate(q)
							elapsed := b.Elapsed() - start
							if err != nil {
								b.Fatalf("evaluate %q: %v", q, err)
							}
							nsQuerySamples = append(nsQuerySamples, float64(elapsed.Nanoseconds()))
						}
					}

					b.StopTimer()
					meanVal, _ := stats.Mean(nsQuerySamples)
					b.ReportMetric(meanVal, "avg_ns/query")
					b.ReportMetric(ci95(nsQuerySamples), "ci95_ns/query")
				})
			}
		})
	}
}

func BenchmarkSearchRank(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			eng, terms := benchmarkSearchEngine(b, size)

			for _, qt := range makeSearchQueryTypes() {
				b.Run(qt.name, func(b *testing.B) {
					rng := rand.New(rand.NewSource(42))
					queries := make([]string, searchQueriesPerType)
					for i := range queries {
						queries[i] = qt.gen(rng, terms)
					}

					nsQuerySamples := make([]float64, 0, searchQueriesPerType*10)

					for b.Loop() {
						for _, q := range queries {
							start := b.Elapsed()
							_, err := eng.Search(q)
							elapsed := b.Elapsed() - start
							if err != nil {
								b.Fatalf("search %q: %v", q, err)
							}
							nsQuerySamples = append(nsQuerySamples, float64(elapsed.Nanoseconds()))
						}
					}

					b.StopTimer()
					meanVal, _ := stats.Mean(nsQuerySamples)
					b.ReportMetric(meanVal, "avg_ns/query")
					b.ReportMetric(ci95(nsQuerySamples), "ci95_ns/query")
				})
			}
		})
	}
}
