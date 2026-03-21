SIZES ?= 500,1000,2500,5000,7500
METRICS_OUT ?= metrics
BENCH_OUT ?= bench_data.txt
BENCH_TIME ?= 60s
FUZZ_TIME ?= 15s
BENCH_FILTER ?= ^BenchmarkIndex

.PHONY: test fuzz bench clean

test:
	go test ./...

fuzz:
	go test -run '^$$' -fuzz '^FuzzExactDuplicatesAreFound$$' -fuzztime $(FUZZ_TIME) .

bench:
	SIZES='$(SIZES)' go test -run '^$$' -bench '$(BENCH_FILTER)' -benchmem . > '$(BENCH_OUT)' -benchtime '$(BENCH_TIME)'

clean:
	rm -rf metrics. *.txt
