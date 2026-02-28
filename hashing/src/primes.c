#include <stdbool.h>

#include "primes.h"


static bool is_prime(const unsigned long long n)
{
    if (n <= 1) return false;
    if (n <= 3) return true;
    if (n % 2 == 0 || n % 3 == 0) return false;

    for (unsigned long long i = 5; i * i <= n; i += 6)
        if (n % i == 0 || n % (i + 2) == 0) return false;

    return true;
}

unsigned long long find_next_prime(const unsigned long long n)
{
    if (n < 2) return 2;

    unsigned long long candidate = (n + 1) % 2 == 0 ? n + 2 : n + 1;
    while (!is_prime(candidate))
        candidate += 2;

    return candidate;
}
