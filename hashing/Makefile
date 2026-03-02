CC := gcc
CFLAGS := -I inc/ -O2 -Wall -Wextra -Werror

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

leaks:
	-leaks -atExit -- ./a.out data/train_data_version1_10.csv

test_build:
	@test -f /opt/homebrew/include/check.h -o -f /usr/local/include/check.h -o -n "$(shell pkg-config --cflags check 2>/dev/null)" || \
		(echo "libcheck not found" && exit 1)
	$(CC) $(CFLAGS) $(CHECK_CFLAGS) tests/perfect.c src/perfect.c src/hash.c src/primes.c -o perfect_test $(CHECK_LIBS)

test: test_build
	./perfect_test

clean:
	-rm -f a.out perfect_test && rm -rf a.out.dSYM perfect_test.dSYM