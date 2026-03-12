CC := gcc
CFLAGS := -I inc/ -O2 -Wall -Wextra -Werror
BENCH_OUT := data/bench_results_$(shell date +%Y%m%d_%H%M%S).csv
BENCH_CFLAGS := -I inc/ -O3 -ffast-math -DNDEBUG -march=native -mtune=native -flto

CHECK_CFLAGS := $(shell pkg-config --cflags check 2>/dev/null)
CHECK_LIBS := $(shell pkg-config --libs check 2>/dev/null)

ifeq ($(strip $(CHECK_CFLAGS)),)
CHECK_CFLAGS := -I/opt/homebrew/include -I/usr/local/include
endif

ifeq ($(strip $(CHECK_LIBS)),)
CHECK_LIBS := -L/opt/homebrew/lib -L/usr/local/lib -lcheck -lm -pthread
endif

debug:
	$(CC) $(CFLAGS) src/perfect.c src/main.c src/hash.c src/primes.c

run: debug
	./a.out data/train_data_version1.csv

bench_data:
	python3 scripts/generate_bench_data.py --out-dir data/bench --key-len 32

bench_build:
	$(CC) $(BENCH_CFLAGS) src/benchmark.c src/perfect.c src/hash.c src/primes.c -o bench.out

bench: bench_build
	./bench.out $(FILES) > $(BENCH_OUT)

bench_plot:
	gnuplot -e "input='$(CSV)';outdir='data'" scripts/plot_bench.gnuplot

leaks:
	-leaks -atExit -- ./a.out data/train_data_version1_10.csv

test_build:
	@test -f /opt/homebrew/include/check.h -o -f /usr/local/include/check.h -o -n "$(shell pkg-config --cflags check 2>/dev/null)" || \
		(echo "libcheck not found" && exit 1)
	$(CC) $(CFLAGS) $(CHECK_CFLAGS) tests/perfect.c src/perfect.c src/hash.c src/primes.c -o perfect_test $(CHECK_LIBS)

test: test_build
	./perfect_test

clean:
	-rm -f a.out perfect_test bench.out && rm -rf a.out.dSYM perfect_test.dSYM bench.out.dSYM