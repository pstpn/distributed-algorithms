DATA_RAW       := data/raw
DATA_DOCS      := data/documents

PROFILE_SIZE   ?= 10000
PROFILE_QUERY  ?= cat AND dog OR hello NEAR/5 world
PROFILE_ROUNDS ?= 1000

BENCH_SIZES    ?= 10000,25000,50000,75000,100000,125000,150000,200000
BUILD_SIZES    ?= 500,700,1000,2000,3000,4000
BENCH_RAW_DIR  := data/bench
BENCH_PLOT_DIR := data/results

build:
	go build ./...

test:
	go test ./internal/... -v

test-short:
	go test ./internal/... -short

tui:
	go run ./cmd/tui/ -data $(DATA_DOCS)

parse:
	go run ./scripts/parse_wiki/ -input $(DATA_RAW) -output $(DATA_DOCS)

profile-build:
	mkdir -p data/profiles
	go run ./cmd/profile \
		-mode build \
		-size $(PROFILE_SIZE) \
		-rounds $(PROFILE_ROUNDS) \
		-cpuprofile data/profiles/build.pprof

profile-search:
	mkdir -p data/profiles
	go run ./cmd/profile \
		-mode search \
		-size $(PROFILE_SIZE) \
		-query '$(PROFILE_QUERY)' \
		-rounds $(PROFILE_ROUNDS) \
		-cpuprofile data/profiles/search.pprof

render-build:
	mkdir -p data/results
	./scripts/render_profiles.sh data/profiles/build.pprof data/results/build.pdf

render-search:
	mkdir -p data/results
	./scripts/render_profiles.sh data/profiles/search.pprof data/results/search.pdf

bench-collect:
	mkdir -p $(BENCH_RAW_DIR) $(BENCH_PLOT_DIR)
	BUILD_SIZES='$(BUILD_SIZES)' SIZES='$(BENCH_SIZES)' go test -run '^$$' -bench 'Benchmark' ./internal/engine/ -timeout 1m > $(BENCH_RAW_DIR)/benchmarks.txt
	awk -v raw_dir='$(BENCH_RAW_DIR)' -f scripts/collect_bench.awk $(BENCH_RAW_DIR)/benchmarks.txt > $(BENCH_RAW_DIR)/benchmarks.csv

bench-plot:
	mkdir -p $(BENCH_PLOT_DIR)
	gnuplot -e "raw_dir='$(BENCH_RAW_DIR)'; plot_dir='$(BENCH_PLOT_DIR)'" scripts/plot_bench.gnuplot

bench: bench-collect bench-plot

compression:
	mkdir -p $(BENCH_RAW_DIR) $(BENCH_PLOT_DIR)
	go run ./cmd/compression -data $(DATA_DOCS) -output $(BENCH_RAW_DIR) -sizes '$(BENCH_SIZES)'

report:
	cd report && /usr/local/bin/pdflatex -synctex=1 -interaction=nonstopmode "report".tex >/dev/null

.PHONY: test tui parse profile-build profile-search render-build render-search build bench-collect bench-plot bench compression report
