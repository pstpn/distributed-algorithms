SIZES ?= 500,1000,5000,10000
PROFILE_OP ?= build
PROFILE_SIZE ?= 10000
PROFILE_OUT ?= metrics/profiles
METRICS_OUT ?= metrics
FUZZ_TIME ?= 15s
BENCH_FILTER ?= ^BenchmarkIndex
THRESHOLD ?= 0.5
NUM_HASHES ?= 64
BANDS ?= 8
SHINGLE_SIZE ?= 2

.PHONY: test test-short fuzz bench profile collect metrics plot clean

test:
	go test ./...

test-short:
	go test -short ./...

fuzz:
	go test -run '^$$' -fuzz '^FuzzExactDuplicatesAreFound$$' -fuzztime $(FUZZ_TIME) .

bench:
	SIZES='$(SIZES)' go test -run '^$$' -bench '$(BENCH_FILTER)' -benchmem .

profile:
	go run ./cmd/profile -op $(PROFILE_OP) -size $(PROFILE_SIZE) -threshold $(THRESHOLD) -num-hashes $(NUM_HASHES) -bands $(BANDS) -shingle-size $(SHINGLE_SIZE) -out $(PROFILE_OUT)

collect:
	go run ./cmd/collectmetrics -sizes $(SIZES) -threshold $(THRESHOLD) -num-hashes $(NUM_HASHES) -bands $(BANDS) -shingle-size $(SHINGLE_SIZE) -out $(METRICS_OUT)

metrics: collect

plot:
	gnuplot -e "raw_dir='$(METRICS_OUT)/raw'; plot_dir='$(METRICS_OUT)/plots'" plot_metrics.gnuplot

clean:
	rm -rf metrics
