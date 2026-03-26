CC := gcc
CFLAGS := -I inc/ -O2 -Wall -Wextra -Werror
BENCH_OUT := data/bench_results_$(shell date +%Y%m%d_%H%M%S).csv
LATEST_BENCH_CSV := $(shell ls -1t data/bench_results_*.csv 2>/dev/null | head -n 1)
BENCH_CFLAGS := -I inc/ -O3 -ffast-math -DNDEBUG -march=native -mtune=native -flto

CHECK_CFLAGS := $(shell pkg-config --cflags check 2>/dev/null)
CHECK_LIBS := $(shell pkg-config --libs check 2>/dev/null)

ifeq ($(strip $(CHECK_CFLAGS)),)
CHECK_CFLAGS := -I/opt/homebrew/include -I/usr/local/include
endif

ifeq ($(strip $(CHECK_LIBS)),)
CHECK_LIBS := -L/opt/homebrew/lib -L/usr/local/lib -lcheck -lm -pthread
endif

.PHONY: bench_data
bench_data:
	python3 scripts/generate_bench_data.py --out-dir data/bench

.PHONY: bench_build
bench_build:
	$(CC) $(BENCH_CFLAGS) src/benchmark.c src/kdtree.c -o bench.out -lm

.PHONY: bench_data
bench: bench_build
	./bench.out > $(BENCH_OUT)

.PHONY: plot
plot:
	gnuplot -e "input='$(LATEST_BENCH_CSV)';outdir='data'" scripts/plot_bench.gnuplot

.PHONY: test_build
test_build:
	@test -f /opt/homebrew/include/check.h -o -f /usr/local/include/check.h -o -n "$(shell pkg-config --cflags check 2>/dev/null)" || \
		(echo "libcheck not found" && exit 1)
	$(CC) $(CFLAGS) $(CHECK_CFLAGS) tests/kdtree.c src/kdtree.c -o kdtree_test $(CHECK_LIBS)

.PHONY: test
test: test_build
	./kdtree_test

.PHONY: clean
clean:
	-rm -f a.out kdtree_test bench.out && rm -rf a.out.dSYM kdtree_test.dSYM bench.out.dSYM

.PHONY: report
report:
	cd report && /usr/local/bin/pdflatex -synctex=1 -interaction=nonstopmode "report".tex >/dev/null
