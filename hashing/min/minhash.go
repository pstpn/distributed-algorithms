package minhash

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	DefaultNumHashes           = 64
	DefaultBands               = 8
	DefaultShingleSize         = 2
	DefaultSimilarityThreshold = 0.8
)

type Config struct {
	NumHashes           int
	Bands               int
	ShingleSize         int
	SimilarityThreshold float64
}

type Document struct {
	ID   string
	Text string
}

type Match struct {
	ID    string
	Score float64
}

type Stats struct {
	DocumentCount int
	BucketCount   int
	BandCount     int
	NumHashes     int
	ShingleSize   int
}

type Index struct {
	cfg     Config
	seeds   []uint64
	docs    map[string]storedDocument
	buckets []map[uint64]map[string]struct{}
}

type storedDocument struct {
	Text      string
	Signature []uint64
	Shingles  map[uint64]struct{}
}

func DefaultConfig() Config {
	return Config{
		NumHashes:           DefaultNumHashes,
		Bands:               DefaultBands,
		ShingleSize:         DefaultShingleSize,
		SimilarityThreshold: DefaultSimilarityThreshold,
	}
}

func NewIndex(cfg Config) (*Index, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	idx := &Index{
		cfg:     cfg,
		seeds:   generateSeeds(cfg.NumHashes),
		docs:    make(map[string]storedDocument),
		buckets: make([]map[uint64]map[string]struct{}, cfg.Bands),
	}
	for i := range idx.buckets {
		idx.buckets[i] = make(map[uint64]map[string]struct{})
	}
	return idx, nil
}

func (cfg Config) validate() error {
	if cfg.NumHashes <= 0 {
		return errors.New("num hashes must be positive")
	}
	if cfg.Bands <= 0 {
		return errors.New("bands must be positive")
	}
	if cfg.ShingleSize <= 0 {
		return errors.New("shingle size must be positive")
	}
	if cfg.NumHashes%cfg.Bands != 0 {
		return fmt.Errorf("num hashes (%d) must be divisible by bands (%d)", cfg.NumHashes, cfg.Bands)
	}
	if cfg.SimilarityThreshold < 0 || cfg.SimilarityThreshold > 1 {
		return errors.New("similarity threshold must be in [0, 1]")
	}
	return nil
}

func (idx *Index) Build(docs []Document) error {
	for _, doc := range docs {
		if err := idx.Add(doc); err != nil {
			return err
		}
	}
	return nil
}

func (idx *Index) Add(doc Document) error {
	if doc.ID == "" {
		return errors.New("document id must not be empty")
	}
	if _, exists := idx.docs[doc.ID]; exists {
		return fmt.Errorf("document %q already exists", doc.ID)
	}
	shingles := shingleHashes(doc.Text, idx.cfg.ShingleSize)
	signature := idx.signatureFor(shingles)
	idx.docs[doc.ID] = storedDocument{
		Text:      doc.Text,
		Signature: signature,
		Shingles:  shingles,
	}
	rowsPerBand := idx.rowsPerBand()
	for band := 0; band < idx.cfg.Bands; band++ {
		start := band * rowsPerBand
		end := start + rowsPerBand
		bucketKey := bandKey(signature[start:end])
		bucket := idx.buckets[band][bucketKey]
		if bucket == nil {
			bucket = make(map[string]struct{})
			idx.buckets[band][bucketKey] = bucket
		}
		bucket[doc.ID] = struct{}{}
	}
	return nil
}

func (idx *Index) CandidateIDs(text string) []string {
	shingles := shingleHashes(text, idx.cfg.ShingleSize)
	signature := idx.signatureFor(shingles)
	rowsPerBand := idx.rowsPerBand()
	set := make(map[string]struct{})
	for band := 0; band < idx.cfg.Bands; band++ {
		start := band * rowsPerBand
		end := start + rowsPerBand
		bucketKey := bandKey(signature[start:end])
		for id := range idx.buckets[band][bucketKey] {
			set[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (idx *Index) FindDuplicates(text string, threshold float64) []Match {
	threshold = idx.resolveThreshold(threshold)
	queryShingles := shingleHashes(text, idx.cfg.ShingleSize)
	ids := idx.CandidateIDs(text)
	matches := make([]Match, 0, len(ids))
	for _, id := range ids {
		doc := idx.docs[id]
		score := jaccard(queryShingles, doc.Shingles)
		if score >= threshold {
			matches = append(matches, Match{ID: id, Score: score})
		}
	}
	return sortMatches(matches)
}

func (idx *Index) FullScanDuplicates(text string, threshold float64) []Match {
	threshold = idx.resolveThreshold(threshold)
	queryShingles := shingleHashes(text, idx.cfg.ShingleSize)
	matches := make([]Match, 0)
	for id, doc := range idx.docs {
		score := jaccard(queryShingles, doc.Shingles)
		if score >= threshold {
			matches = append(matches, Match{ID: id, Score: score})
		}
	}
	return sortMatches(matches)
}

func (idx *Index) Stats() Stats {
	bucketCount := 0
	for _, band := range idx.buckets {
		bucketCount += len(band)
	}
	return Stats{
		DocumentCount: len(idx.docs),
		BucketCount:   bucketCount,
		BandCount:     idx.cfg.Bands,
		NumHashes:     idx.cfg.NumHashes,
		ShingleSize:   idx.cfg.ShingleSize,
	}
}

func (idx *Index) rowsPerBand() int {
	return idx.cfg.NumHashes / idx.cfg.Bands
}

func (idx *Index) resolveThreshold(threshold float64) float64 {
	if threshold == 0 {
		return idx.cfg.SimilarityThreshold
	}
	if threshold < 0 {
		return 0
	}
	if threshold > 1 {
		return 1
	}
	return threshold
}

func (idx *Index) signatureFor(shingles map[uint64]struct{}) []uint64 {
	signature := make([]uint64, idx.cfg.NumHashes)
	for i := range signature {
		signature[i] = math.MaxUint64
		seed := idx.seeds[i]
		for shingle := range shingles {
			value := mix64(shingle ^ seed)
			if value < signature[i] {
				signature[i] = value
			}
		}
	}
	return signature
}

func shingleHashes(text string, shingleSize int) map[uint64]struct{} {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return map[uint64]struct{}{hashString64("<empty>"): {}}
	}
	if len(tokens) < shingleSize {
		return map[uint64]struct{}{hashString64(strings.Join(tokens, " ")): {}}
	}
	set := make(map[uint64]struct{}, len(tokens)-shingleSize+1)
	for i := 0; i+shingleSize <= len(tokens); i++ {
		set[hashString64(strings.Join(tokens[i:i+shingleSize], " "))] = struct{}{}
	}
	return set
}

func tokenize(text string) []string {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return unicode.ToLower(r)
		default:
			return ' '
		}
	}, text)
	return strings.Fields(normalized)
}

func jaccard(left map[uint64]struct{}, right map[uint64]struct{}) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1
	}
	intersection := 0
	for value := range left {
		if _, ok := right[value]; ok {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 1
	}
	return float64(intersection) / float64(union)
}

func sortMatches(matches []Match) []Match {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Score > matches[j].Score
	})
	return matches
}

func generateSeeds(count int) []uint64 {
	state := uint64(0x9e3779b97f4a7c15)
	seeds := make([]uint64, count)
	for i := 0; i < count; i++ {
		state += 0x9e3779b97f4a7c15
		seeds[i] = mix64(state)
	}
	return seeds
}

func bandKey(values []uint64) uint64 {
	h := uint64(1469598103934665603)
	for _, value := range values {
		h ^= mix64(value)
		h *= 1099511628211
	}
	return h
}

func hashString64(value string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return h.Sum64()
}

func mix64(value uint64) uint64 {
	value ^= value >> 30
	value *= 0xbf58476d1ce4e5b9
	value ^= value >> 27
	value *= 0x94d049bb133111eb
	value ^= value >> 31
	return value
}
