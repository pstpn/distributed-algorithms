leaks:
	-leaks -atExit -- ./a.out data/train_data_version1.csv

debug:
	gcc -I inc/ -g src/perfect.c src/main.c src/hash.c src/primes.c

run: debug
	./a.out data/train_data_version1.csv

clean:
	-rm a.out && rm -rf a.out.dSYM