SHELL := /bin/sh
BENCH ?= BenchmarkMap
BENCH_TIME ?= 1s
BENCH_SIZES ?= 1000,2500,5000,7500,10000,25000,50000,100000

PROFILE_MODE ?= parallel
PROFILE_OP ?= get
PROFILE_SIZE ?= 50000
PROFILE_ROUNDS ?= 100
PROFILE_PROCS ?= 0
PROFILE_WORKERS ?= 0
PROFILE_BUCKETS ?= 0

PROFILE_OUT ?= data/profiles/cpu.pprof
PROFILE_PDF ?= results/profiles/cpu.pdf

.PHONY: test bench plot profile clean report

test:
	go test -race -count=1 ./...

bench:
	BENCH_SIZES=$(BENCH_SIZES) go test -run ^$$ -bench $(BENCH) -benchtime $(BENCH_TIME) -benchmem

plot:
	mkdir -p results
	gnuplot scripts/plot.gnuplot

profile:
	mkdir -p $(dir $(PROFILE_OUT)) $(dir $(PROFILE_PDF))
	go run ./cmd/profile \
		-mode $(PROFILE_MODE) \
		-op $(PROFILE_OP) \
		-size $(PROFILE_SIZE) \
		-rounds $(PROFILE_ROUNDS) \
		-procs $(PROFILE_PROCS) \
		-workers $(PROFILE_WORKERS) \
		-buckets $(PROFILE_BUCKETS) \
		-cpuprofile $(PROFILE_OUT)
	./scripts/render_profiles.sh $(PROFILE_OUT) $(PROFILE_PDF)

clean:
	rm -f data/benchmarks.csv
	rm -f data/profiles/*.pprof results/*.pdf results/profiles/*.pdf results/profiles/*.dot

report:
	cd report && /usr/local/bin/pdflatex -synctex=1 -interaction=nonstopmode "report".tex >/dev/null